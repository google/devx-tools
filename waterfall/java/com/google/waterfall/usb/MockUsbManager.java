package com.google.waterfall.usb;

import android.app.PendingIntent;
import android.content.Context;
import android.content.Intent;
import android.hardware.usb.UsbDevice;
import android.hardware.usb.UsbDeviceConnection;
import android.net.LocalSocket;
import android.net.LocalSocketAddress;
import android.os.ParcelFileDescriptor;
import android.util.Log;
import java.io.IOException;
import java.util.Arrays;
import java.util.HashMap;

/** Mock implementation of UsbManager class */
public class MockUsbManager implements UsbManagerIntf {

  private static final String ACTION_USB_PERMISSION =
      "com.google.android.waterfall.usb.USB_PERMISSION";

  private static final String TAG = "waterfall.UsbService.MockUsbManager";

  public static final String SOCKET_NAME = "waterfall_mock_usb";

  private final MockUsbAccessory accessory = new MockUsbAccessory();

  private Context context;
  private ParcelFileDescriptor pfd;
  private boolean hasPermission;
  private boolean grantPermission;

  public UsbAccessoryIntf[] accessories = new UsbAccessoryIntf[] {accessory};

  public MockUsbManager(Context context) {
    Log.i(TAG, "new MockUsbManager");
    this.context = context;
  }

  @Override
  public UsbAccessoryIntf[] getAccessoryList() {
    Log.i(TAG, "getAccessoryList " + Arrays.toString(accessories));
    return accessories;
  }

  public void setAccessoryList(UsbAccessoryIntf[] accessories) {
    Log.i(TAG, "setAccessoryList " + Arrays.toString(accessories));
    this.accessories = accessories;
  }

  @Override
  public HashMap<String, UsbDevice> getDeviceList() {
    return new HashMap<String, UsbDevice>();
  }

  @Override
  public boolean hasPermission(UsbAccessoryIntf accessory) {
    return hasPermission;
  }

  @Override
  public boolean hasPermission(UsbDevice device) {
    return false;
  }

  @Override
  public ParcelFileDescriptor openAccessory(UsbAccessoryIntf accessory) {
    Log.i(TAG, "Opening LocalSocket accessory");
    LocalSocket ls = new LocalSocket();
    try {
      if (pfd != null) {
        return pfd;
      }

      // Bind to a local socket. The test driver is responsible to open up this socket before
      // starting the test.
      Log.i(TAG, "Binding LocalSocket accessory");
      ls.connect(new LocalSocketAddress(SOCKET_NAME));
      Log.i(TAG, "Accessory bound to " + SOCKET_NAME);
      pfd = ParcelFileDescriptor.dup(ls.getFileDescriptor());
      return pfd;

    } catch (IOException e) {
      throw new RuntimeException(e);
    } finally {
      closeSocket(ls);
    }
  }

  private void closeSocket(LocalSocket ls) {
    if (ls == null) {
      return;
    }
    try {
      ls.close();
    } catch (IOException e) {
      throw new RuntimeException(e);
    }
  }

  @Override
  public UsbDeviceConnection openDevice(UsbDevice device) {
    throw new RuntimeException("method not implemented");
  }

  @Override
  public void requestPermission(UsbAccessoryIntf accessory, PendingIntent pi) {
    if (!grantPermission) {
      return;
    }

    setHasPermission(true);
    Intent intent = new Intent();
    intent.setAction(ACTION_USB_PERMISSION);
    context.sendBroadcast(intent);
  }

  public void setGrantPermission(boolean grant) {
    grantPermission = grant;
  }

  public void setHasPermission(boolean hasPermission) {
    this.hasPermission = hasPermission;
  }

  @Override
  public void requestPermission(UsbDevice device, PendingIntent pi) {}
}
