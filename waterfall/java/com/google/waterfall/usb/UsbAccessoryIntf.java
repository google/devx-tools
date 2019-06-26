package com.google.waterfall.usb;

import android.hardware.usb.UsbAccessory;

/** UsbAccessoryIntf defines the methods required to manage a USB connection. */
public interface UsbAccessoryIntf {

  public int describeContents();

  public String getDescription();

  public String getManufacturer();

  public String getSerial();

  public String geUri();

  public String getVersion();

  public UsbAccessory getAccessory();
}
