package com.google.waterfall.usb;

import android.app.Notification;
import android.app.PendingIntent;
import android.app.Service;
import android.content.BroadcastReceiver;
import android.content.Context;
import android.content.Intent;
import android.content.IntentFilter;
import android.hardware.usb.UsbManager;
import android.os.Binder;
import android.os.IBinder;
import android.os.ParcelFileDescriptor;
import android.support.annotation.NonNull;
import androidx.core.app.NotificationCompat;
import android.util.Log;
import androidx.annotation.VisibleForTesting;
import dagger.Component;
import java.io.FileInputStream;
import java.io.FileOutputStream;
import java.io.IOException;
import java.io.InputStream;
import java.io.OutputStream;
import java.net.Socket;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.Future;
import javax.inject.Inject;
import javax.inject.Singleton;

public final class UsbService extends Service {

  @Singleton
  @Component(modules = AndroidUsbModule.class)
  public interface ServiceComponent {
    void inject(UsbService usbService);
  }

  private static final String TAG = "waterfall.UsbService";

  private static final String ACTION_USB_PERMISSION =
      "com.google.android.waterfall.usb.USB_PERMISSION";

  private final IBinder localBinder = new LocalBinder();

  // Read/Writer threads + driver thread
  private static final int NUM_THREADS = 3;
  private static ExecutorService IO_EXECUTOR = Executors.newFixedThreadPool(NUM_THREADS);

  private static final String WATERFALL_PORT_KEY = "waterfallPort";
  private static final short DEFAULT_WATERFALL_PORT = 8088;

  private static final int DEFAULT_BUFFER_SIZE = 1024;
  private static final String BUFFER_SIZE_KEY = "bufferSize";

  private ServiceComponent component;

  @VisibleForTesting @Inject UsbManagerIntf usbManager;

  private UsbAccessoryIntf accessory;
  private CountDownLatch accessoryAttachedBarrier = new CountDownLatch(1);
  private CountDownLatch permissionBarrier = new CountDownLatch(1);
  private ParcelFileDescriptor accessoryFd;

  private boolean usbReceiverRegistered = false;
  private boolean usbDisconnectRegistered = false;

  // This can also be a callable which simplifies exception handling
  private static class CopyRunnable implements Runnable {
    private InputStream is;
    private OutputStream os;
    private int bufferSize;

    public CopyRunnable(@NonNull InputStream is, @NonNull OutputStream os, int bufferSize) {
      this.is = is;
      this.os = os;
      this.bufferSize = checkArg(bufferSize, bufferSize > 0);
    }

    @Override
    public void run() {
      try {
        copy();
      } catch (IOException e) {
        throw new RuntimeException(e);
      }
    }

    private void copy() throws IOException {
      byte buffer[] = new byte[bufferSize];
      while (true) {
        int r = is.read(buffer);
        if (r == -1) {
          break;
        }
        os.write(buffer, 0, r);
      }
    }
  }

  private BroadcastReceiver usbDisconnectReceiver =
      usbDisconnectReceiver =
          new BroadcastReceiver() {
            @Override
            public void onReceive(Context context, Intent intent) {
              String action = intent.getAction();
              if (action.equals(UsbManager.ACTION_USB_ACCESSORY_DETACHED)) {
                if (accessory != null) {
                  tearDown();
                }
              }
            }
          };

  private final BroadcastReceiver usbReceiver =
      new BroadcastReceiver() {
        @Override
        public void onReceive(Context context, Intent intent) {
          if (ACTION_USB_PERMISSION.equals(intent.getAction())) {
            Log.i(TAG, "Permission granted");
            permissionBarrier.countDown();
          }
        }
      };

  @Override
  public void onCreate() {
    super.onCreate();

    Log.i(TAG, "onCreate");
    component =
        DaggerUsbService_ServiceComponent.builder()
            .androidUsbModule(new AndroidUsbModule(this))
            .build();
    component.inject(this);
  }

  @Override
  public void onDestroy() {
    Log.i(TAG, "onDestroy");
    tearDown();
    super.onDestroy();
  }

  @Override
  public void onStart(Intent intent, int id) {
    if (UsbManager.ACTION_USB_ACCESSORY_ATTACHED.equals(intent.getAction())) {
      // Unfortunately we cant process this event. We need extra information
      Log.i(TAG, "Received ACTION_USB_ACCESSORY_ATTACHED intent.");
      return;
    }
    Log.i(TAG, "onStart");

    registerReceiver(usbReceiver, new IntentFilter(ACTION_USB_PERMISSION));
    usbReceiverRegistered = true;

    startService(
        intent.getShortExtra(WATERFALL_PORT_KEY, DEFAULT_WATERFALL_PORT),
        KB(intent.getIntExtra(BUFFER_SIZE_KEY, DEFAULT_BUFFER_SIZE)));
  }

  @Override
  public int onStartCommand(Intent intent, int flags, int startId) {
    Log.i(TAG, "onStartCommand");
    onStart(intent, startId);
    return START_NOT_STICKY;
  }

  @Override
  public IBinder onBind(Intent intent) {
    // NOTE: this only works when client and service run on the same process.
    // It is intended for tests only.
    return localBinder;
  }

  private void startService(short waterfallPort, int bufferSize) {
    Intent launcher = new Intent(this, UsbService.class);
    launcher.setAction(Intent.ACTION_MAIN);
    launcher.addCategory(Intent.CATEGORY_LAUNCHER);
    launcher.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK);
    Notification notif =
        new NotificationCompat.Builder(this)
            .setSmallIcon(R.drawable.ic_shortcut_axt_logo)
            .setContentTitle("Waterfall")
            .setContentText("Waterfall USB service")
            .setContentIntent(
                PendingIntent.getActivity(
                    getApplicationContext(), 0, launcher, PendingIntent.FLAG_UPDATE_CURRENT))
            .build();
    startForeground(R.id.service_foreground_notification, notif);

    // Dispatch two threads to do the actual work.
    // Thread 1 waits for an accessory to be attached and sets it up.
    // Thread 2 waits for USB configuration to be done and proxies data to waterfall.
    Future<?> c =
        IO_EXECUTOR.submit(
            new Runnable() {
              @Override
              public void run() {
                try {
                  configureUsbAccessory();
                } catch (InterruptedException e) {
                  throw new RuntimeException(e);
                }
              }
            });

    Future<?> r =
        IO_EXECUTOR.submit(
            new Runnable() {
              @Override
              public void run() {
                try {
                  Log.i(TAG, "Waiting for permission ...");
                  permissionBarrier.await();
                  Log.i(TAG, "Permission granted ...");

                  // We now have permission to open the USB device. Go ahead and connect.
                  accessoryFd = connectToUsbAccessory(getAccessory());
                  proxy(waterfallPort, bufferSize, accessoryFd);
                } catch (InterruptedException e) {
                  throw new RuntimeException(e);
                }
              }
            });
  }

  private ParcelFileDescriptor connectToUsbAccessory(@NonNull UsbAccessoryIntf accessory) {
    registerReceiver(
        usbDisconnectReceiver, new IntentFilter(UsbManager.ACTION_USB_ACCESSORY_DETACHED));
    usbDisconnectRegistered = true;

    // This method should only be called after having acquired permission to use the USB device.
    if (!usbManager.hasPermission(accessory)) {
      throw new RuntimeException("attempting to connect to USB accessory without permission.");
    }

    ParcelFileDescriptor fd = usbManager.openAccessory(accessory);
    if (fd == null) {
      Log.e(TAG, "Got null file descriptor when opening accessory.");
      throw new RuntimeException("Null file descriptor for accessory!");
    }
    return fd;
  }

  private void proxy(short port, int bufferSize, ParcelFileDescriptor accessoryFd) {
    try {
      Log.i(TAG, "Connecting to waterfall");
      Socket wtfSocket = new Socket("localhost", port);
      Log.i(TAG, "Connected to waterfall");

      // Disable flow control. Connection is always on loopback.
      wtfSocket.setTcpNoDelay(true);

      OutputStream outWtf = wtfSocket.getOutputStream();
      InputStream inWtf = wtfSocket.getInputStream();
      OutputStream outUsb = new FileOutputStream(accessoryFd.getFileDescriptor());
      InputStream inUsb = new FileInputStream(accessoryFd.getFileDescriptor());

      // usb -> waterfall: thread
      Future<?> r = IO_EXECUTOR.submit(new CopyRunnable(inUsb, outWtf, bufferSize));

      // waterfall -> usb : thread
      Future<?> w = IO_EXECUTOR.submit(new CopyRunnable(inWtf, outUsb, bufferSize));

      r.get();
      w.get();
    } catch (Exception e) {
      throw new RuntimeException(e);
    }
  }

  private void configureUsbAccessory() throws InterruptedException {
    Log.i(TAG, "configureUsbAccessory");
    UsbAccessoryIntf[] accessories = usbManager.getAccessoryList();

    if (accessories == null) {
      Log.i(TAG, "Waiting for accessory to attach");
      accessoryAttachedBarrier.await();
    }

    if (accessories == null || accessories.length != 1) {
      throw new RuntimeException("Unexpected number of USB accessories.");
    }

    accessory = accessories[0];
    if (usbManager.hasPermission(accessory)) {
      permissionBarrier.countDown();
      return;
    }
    usbManager.requestPermission(
        accessory, PendingIntent.getBroadcast(this, 0, new Intent(ACTION_USB_PERMISSION), 0));
  }

  private UsbAccessoryIntf getAccessory() {
    if (accessory == null) {
      throw new RuntimeException("Null accessory.");
    }
    return accessory;
  }

  private void tearDown() {
    Log.i(TAG, "tearDown()");
    if (accessoryFd != null) {
      try {
        accessoryFd.close();
      } catch (IOException e) {
        // ignore
      }
      accessoryFd = null;
    }
    if (usbDisconnectRegistered) {
      usbDisconnectRegistered = false;
      unregisterReceiver(usbDisconnectReceiver);
    }
    if (usbReceiverRegistered) {
      usbReceiverRegistered = false;
      unregisterReceiver(usbReceiver);
    }

    IO_EXECUTOR.shutdown();
  }

  private static int KB(int sizeInBytes) {
    return sizeInBytes * 1024;
  }

  private static <T> T checkArg(T arg, boolean condition) {
    if (!condition) {
      throw new IllegalArgumentException();
    }
    return arg;
  }

  /**
   * Gets the service instance through the IBinder. This allows bound clients in the same process to
   * interact with the service. NOTE: this is only intented for testing purposes. Instrumentation
   * and app running under the same process.
   */
  public class LocalBinder extends Binder {
    public UsbService getService() {
      return UsbService.this;
    }
  }
}
