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
import android.util.Log;
import androidx.annotation.VisibleForTesting;
import androidx.core.app.NotificationCompat;
import dagger.Component;
import java.io.FileInputStream;
import java.io.FileOutputStream;
import java.io.IOException;
import java.io.InputStream;
import java.io.OutputStream;
import java.net.Socket;
import java.util.concurrent.ConcurrentLinkedQueue;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.Future;
import javax.inject.Inject;
import javax.inject.Singleton;

/** Android service to get a handle to the USB device */
public final class UsbService extends Service {

  /** Dagger component */
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

  private static final ExecutorService IO_EXECUTOR = Executors.newFixedThreadPool(NUM_THREADS);

  private static final ConcurrentLinkedQueue<Future> TASK_QUEUE = new ConcurrentLinkedQueue();

  private final Object accessoryAttachedLock = new Object();

  private static final String ACTION_PROXY = "proxy";
  private static final String ACTION_ECHO = "echo";
  private static final String WATERFALL_PORT_KEY = "waterfallPort";
  private static final short DEFAULT_WATERFALL_PORT = 8088;

  private static final int DEFAULT_BUFFER_SIZE = 1024;
  private static final String BUFFER_SIZE_KEY = "bufferSize";

  private static boolean serviceStarted = false;
  private static boolean connectionStarted = false;

  private ServiceComponent component;

  @VisibleForTesting @Inject UsbManagerIntf usbManager;

  private UsbAccessoryIntf accessory;

  private CountDownLatch permissionBarrier = new CountDownLatch(1);
  private ParcelFileDescriptor accessoryFd;

  private boolean usbReceiverRegistered = false;
  private boolean usbDisconnectRegistered = false;

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
      new BroadcastReceiver() {
        @Override
        public void onReceive(Context context, Intent intent) {
          String action = intent.getAction();
          if (action.equals(UsbManager.ACTION_USB_ACCESSORY_DETACHED)) {
            Log.i(TAG, "Received accessory detached");
            if (accessory != null) {
              stopSelf();
            }
          }
        }
      };

  private final BroadcastReceiver usbReceiver =
      new BroadcastReceiver() {
        @Override
        public void onReceive(Context context, Intent intent) {
          if (ACTION_USB_PERMISSION.equals(intent.getAction())) {
            Log.i(TAG, "Received permission");
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
  public int onStartCommand(Intent intent, int flags, int startId) {
    Log.i(TAG, "onStartCommand");
    onStart(intent, startId);
    return START_NOT_STICKY;
  }

  @Override
  public void onStart(Intent intent, int id) {
    Log.i(TAG, "onStart with intent " + intent.getAction());

    startService();
    if (intent.getAction() != null) {
      connectService(
          intent.getAction(),
          intent.getShortExtra(WATERFALL_PORT_KEY, DEFAULT_WATERFALL_PORT),
          kb(intent.getIntExtra(BUFFER_SIZE_KEY, DEFAULT_BUFFER_SIZE)));
    }

    if (intent.hasExtra(UsbManager.EXTRA_ACCESSORY)) {
      notifyAccessoryAttached();
    }
    Log.i(TAG, "Done onStart with intent " + intent.getAction());
  }

  private void notifyAccessoryAttached() {
    Log.d(TAG, "Notifying accessory was attached.");
    synchronized (accessoryAttachedLock) {
      accessoryAttachedLock.notify();
    }
    Log.d(TAG, "Notified accessory was attached.");
  }

  @Override
  public IBinder onBind(Intent intent) {
    // NOTE: this only works when client and service run on the same process.
    // It is intended for tests only.
    return localBinder;
  }

  private Notification makeForegroundNotification() {
    Intent launcher = new Intent(this, UsbService.class);
    launcher.setAction(Intent.ACTION_MAIN);
    launcher.addCategory(Intent.CATEGORY_LAUNCHER);
    launcher.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK);
    return new NotificationCompat.Builder(this)
        .setSmallIcon(R.drawable.ic_shortcut_axt_logo)
        .setContentTitle(getString(R.string.foreground_notification_content_title))
        .setContentText(getString(R.string.foreground_notification_content_text))
        .setContentIntent(
            PendingIntent.getActivity(
                getApplicationContext(), 0, launcher, PendingIntent.FLAG_UPDATE_CURRENT))
        .build();
  }

  private void startService() {
    if (serviceStarted) {
      return;
    }

    startForeground(R.id.service_foreground_notification, makeForegroundNotification());
    registerReceiver(usbReceiver, new IntentFilter(ACTION_USB_PERMISSION));
    usbReceiverRegistered = true;

    Context context = this;
    Future<?> c =
        submitToQueue(
            new Runnable() {
              @Override
              public void run() {
                synchronized (accessoryAttachedLock) {
                  try {
                    while (true) {
                      UsbAccessoryIntf[] accessories = usbManager.getAccessoryList();
                      if (accessories == null || accessories.length == 0) {
                        Log.i(TAG, "Waiting for accessory to be attached ...");
                        accessoryAttachedLock.wait();
                        Log.i(TAG, "Accessory attached ...");
                        continue;
                      }
                      accessory = accessories[0];
                      usbManager.requestPermission(
                          accessory,
                          PendingIntent.getBroadcast(
                              context, 0, new Intent(ACTION_USB_PERMISSION), 0));
                      break;
                    }
                  } catch (InterruptedException e) {
                    throw new RuntimeException(e);
                  }
                }
              }
            });

    serviceStarted = true;
  }

  private void connectService(String action, short waterfallPort, int bufferSize) {
    if (connectionStarted) {
      return;
    }

    if (!action.equals(ACTION_PROXY) && !action.equals(ACTION_ECHO)) {
      throw new IllegalArgumentException("Not a vailid action: " + action);
    }

    Future<?> r =
        submitToQueue(
            new Runnable() {
              @Override
              public void run() {
                try {
                  Log.i(TAG, "Waiting for permission ...");
                  permissionBarrier.await();
                  Log.i(TAG, "Permission granted ...");

                  // We now have permission to open the USB device. Go ahead and connect.
                  accessoryFd = connectToUsbAccessory(getAccessory());
                  if (action.equals("proxy")) {
                    proxy(waterfallPort, bufferSize, accessoryFd);
                  } else {
                    echo(bufferSize, accessoryFd);
                  }
                } catch (InterruptedException e) {
                  throw new RuntimeException(e);
                }
              }
            });

    connectionStarted = true;
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

    cancelTasks();
    serviceStarted = false;
    connectionStarted = false;
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
      Future<?> r = submitToQueue(new CopyRunnable(inUsb, outWtf, bufferSize));

      // waterfall -> usb : thread
      Future<?> w = submitToQueue(new CopyRunnable(inWtf, outUsb, bufferSize));

      r.get();
      w.get();
    } catch (Exception e) {
      throw new RuntimeException(e);
    }
  }

  private void echo(int bufferSize, ParcelFileDescriptor accessoryFd) {
    try {
      OutputStream outUsb = new FileOutputStream(accessoryFd.getFileDescriptor());
      InputStream inUsb = new FileInputStream(accessoryFd.getFileDescriptor());

      // usb -> waterfall: thread
      Future<?> r = IO_EXECUTOR.submit(new CopyRunnable(inUsb, outUsb, bufferSize));

      r.get();
    } catch (Exception e) {
      throw new RuntimeException(e);
    }
  }

  private static Future<?> submitToQueue(Runnable r) {
    Future<?> f = IO_EXECUTOR.submit(r);
    TASK_QUEUE.add(f);
    return f;
  }

  private static void cancelTasks() {
    for (Future task : TASK_QUEUE) {
      task.cancel(true);
    }
  }

  private static int kb(int sizeInBytes) {
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
