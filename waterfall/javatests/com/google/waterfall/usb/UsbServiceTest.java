package com.google.waterfall.usb;

import static org.junit.Assert.assertTrue;
import static org.mockito.Mockito.any;
import static org.mockito.Mockito.eq;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.times;
import static org.mockito.Mockito.verify;

import android.content.Context;
import android.content.Intent;
import android.hardware.usb.UsbManager;
import android.net.LocalServerSocket;
import android.net.LocalSocket;
import androidx.test.platform.app.InstrumentationRegistry;
import androidx.test.rule.ServiceTestRule;
import java.io.FileInputStream;
import java.io.FileOutputStream;
import java.io.InputStream;
import java.io.OutputStream;
import java.net.ServerSocket;
import java.net.Socket;
import java.util.Arrays;
import java.util.concurrent.Callable;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.Future;
import org.junit.After;
import org.junit.Before;
import org.junit.Rule;
import org.junit.Test;
import org.junit.runner.RunWith;
import org.junit.runners.JUnit4;
import org.mockito.Mockito;

@RunWith(JUnit4.class)
public final class UsbServiceTest {

  @Rule public final ServiceTestRule usbServiceRule = new ServiceTestRule();

  private final Context context = InstrumentationRegistry.getInstrumentation().getTargetContext();

  private final ExecutorService executor = Executors.newFixedThreadPool(2);

  private Intent makeAttachedIntent() {
    Intent i = new Intent(context, UsbService.class);
    i.putExtra(UsbManager.EXTRA_ACCESSORY, "DUMMY");
    return i;
  }

  private Intent makeConnectIntent() {
    Intent i = new Intent(context, UsbService.class);
    i.setAction("proxy");
    return i;
  }

  @Before
  public void setUp() throws Exception {}

  @After
  public void tearDown() throws Exception {
    executor.shutdown();
  }

  @Test
  public void testOpenAccessoryPermissionGranted() throws Exception {
    MockUsbManager usbManager = new MockUsbManager(context);
    usbManager.setGrantPermission(true);
    UsbManagerIntf usbSpy = Mockito.spy(usbManager);
    AndroidUsbModule.usbManager = usbSpy;
    usbServiceRule.startService(makeAttachedIntent());
    usbServiceRule.startService(makeConnectIntent());

    // TODO(mauriciogg): use a better synchronization mechanism
    Thread.sleep(1000);
    verify(usbSpy, times(1)).requestPermission(eq(usbManager.getAccessoryList()[0]), any());
    verify(usbSpy, times(1)).openAccessory(usbManager.getAccessoryList()[0]);
  }

  @Test
  public void testOpenAccessoryPermissionNotGranted() throws Exception {
    MockUsbManager usbManager = new MockUsbManager(context);
    usbManager.setGrantPermission(false);
    UsbManagerIntf usbSpy = Mockito.spy(usbManager);
    AndroidUsbModule.usbManager = usbSpy;
    usbServiceRule.startService(makeAttachedIntent());
    usbServiceRule.startService(makeConnectIntent());

    Thread.sleep(1000);
    verify(usbSpy, times(1)).requestPermission(eq(usbManager.getAccessoryList()[0]), any());
    verify(usbSpy, never()).openAccessory(usbManager.getAccessoryList()[0]);
  }

  @Test
  public void testProxy() throws Exception {
    LocalSocket ls = null;
    Socket s = null;

    try {
      // Start 2 servers:
      // 1) a unix server -> this will fake the USB connection
      // 2) a tcp server -> this will fake the waterfall connection
      // Test communication from unix <---> tcp goes through.
      // Unix server: the mock usb manager connects to this socket.
      Future<LocalSocket> unixSocketFuture =
          executor.submit(
              new Callable<LocalSocket>() {
                public LocalSocket call() throws Exception {
                  return new LocalServerSocket(MockUsbManager.SOCKET_NAME).accept();
                }
              });

      // TCP server: the usb service connects to this socket.
      Future<Socket> tcpSocketFuture =
          executor.submit(
              new Callable<Socket>() {
                public Socket call() throws Exception {
                  return new ServerSocket(8088).accept();
                }
              });

      MockUsbManager usbManager = new MockUsbManager(context);
      usbManager.setGrantPermission(true);
      UsbManagerIntf usbSpy = Mockito.spy(usbManager);
      AndroidUsbModule.usbManager = usbSpy;
      usbServiceRule.startService(makeAttachedIntent());
      usbServiceRule.startService(makeConnectIntent());

      ls = unixSocketFuture.get();
      s = tcpSocketFuture.get();

      InputStream unixIs = new FileInputStream(ls.getFileDescriptor());
      OutputStream unixOs = new FileOutputStream(ls.getFileDescriptor());
      InputStream tcpIs = s.getInputStream();
      OutputStream tcpOs = s.getOutputStream();

      // Write to USB (Unix) and verify on Waterfall (TCP)
      for (int i = 0; i < 5; i++) {
        String outStr = "Hello msg #" + i;
        byte out[] = outStr.getBytes();
        byte in[] = new byte[out.length];
        unixOs.write(out);
        tcpIs.read(in);
        assertTrue(Arrays.equals(out, in));
      }

      // Write to USB (Unix) and verify on Waterfall (TCP)
      for (int i = 0; i < 5; i++) {
        String outStr = "Hello msg #" + i;
        byte out[] = outStr.getBytes();
        byte in[] = new byte[out.length];
        tcpOs.write(out);
        unixIs.read(in);
        assertTrue(Arrays.equals(out, in));
      }
    } finally {
      if (ls != null) {
        ls.close();
      }
      if (s != null) {
        s.close();
      }
    }
  }
}
