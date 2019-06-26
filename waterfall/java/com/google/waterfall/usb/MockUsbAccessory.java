package com.google.waterfall.usb;

import android.hardware.usb.UsbAccessory;

final class MockUsbAccessory implements UsbAccessoryIntf {

  @Override
  public int describeContents() {
    return 0;
  }

  @Override
  public String getDescription() {
    return "";
  }

  @Override
  public String getManufacturer() {
    return "";
  }

  @Override
  public String getSerial() {
    return "";
  }

  @Override
  public String geUri() {
    return "";
  }

  @Override
  public String getVersion() {
    return "";
  }

  @Override
  public UsbAccessory getAccessory() {
    throw new RuntimeException("unimplemented");
  }
}
