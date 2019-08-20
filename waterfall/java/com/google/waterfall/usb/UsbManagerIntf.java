package com.google.waterfall.usb;

import android.app.PendingIntent;
import android.hardware.usb.UsbDevice;
import android.hardware.usb.UsbDeviceConnection;
import android.os.ParcelFileDescriptor;
import java.util.HashMap;

/** Interface exposing UsbManager methods in order to enable mocking */
public interface UsbManagerIntf {

  public UsbAccessoryIntf[] getAccessoryList();

  public HashMap<String, UsbDevice> getDeviceList();

  public boolean hasPermission(UsbAccessoryIntf accessory);

  public boolean hasPermission(UsbDevice device);

  public ParcelFileDescriptor openAccessory(UsbAccessoryIntf accessory);

  public UsbDeviceConnection openDevice(UsbDevice device);

  public void requestPermission(UsbAccessoryIntf accessory, PendingIntent pi);

  public void requestPermission(UsbDevice device, PendingIntent pi);
}
