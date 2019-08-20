package com.google.waterfall.usb;

import android.app.PendingIntent;
import android.hardware.usb.UsbAccessory;
import android.hardware.usb.UsbDevice;
import android.hardware.usb.UsbDeviceConnection;
import android.hardware.usb.UsbManager;
import android.os.ParcelFileDescriptor;
import android.util.Log;
import java.util.HashMap;

/** Real UsbManager implementation */
public class AoaUsbManager implements UsbManagerIntf {

  private static final String TAG = "waterfall.UsbService.AoaUsbManager";

  private UsbManager realManager;

  public AoaUsbManager(UsbManager usbManager) {
    Log.i(TAG, "new AoaUsbManager");
    realManager = usbManager;
  }

  @Override
  public UsbAccessoryIntf[] getAccessoryList() {
    UsbAccessory accessories[] = realManager.getAccessoryList();

    if (accessories == null) {
      return null;
    }

    UsbAccessoryIntf wrappedAccessories[] = new AoaUsbAccessory[accessories.length];

    for (int i = 0; i < accessories.length; i++) {
      wrappedAccessories[i] = new AoaUsbAccessory(accessories[i]);
    }
    return wrappedAccessories;
  }

  @Override
  public HashMap<String, UsbDevice> getDeviceList() {
    return realManager.getDeviceList();
  }

  @Override
  public boolean hasPermission(UsbAccessoryIntf accessory) {
    return realManager.hasPermission(accessory.getAccessory());
  }

  @Override
  public boolean hasPermission(UsbDevice device) {
    return realManager.hasPermission(device);
  }

  @Override
  public ParcelFileDescriptor openAccessory(UsbAccessoryIntf accessory) {
    return realManager.openAccessory(accessory.getAccessory());
  }

  @Override
  public UsbDeviceConnection openDevice(UsbDevice device) {
    return realManager.openDevice(device);
  }

  @Override
  public void requestPermission(UsbAccessoryIntf accessory, PendingIntent pi) {
    realManager.requestPermission(accessory.getAccessory(), pi);
  }

  @Override
  public void requestPermission(UsbDevice device, PendingIntent pi) {
    realManager.requestPermission(device, pi);
  }
}
