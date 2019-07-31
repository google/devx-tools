package com.google.waterfall.usb;

import android.hardware.usb.UsbAccessory;

final class AoaUsbAccessory implements UsbAccessoryIntf {

  private final UsbAccessory realAccessory;

  public AoaUsbAccessory(UsbAccessory accessory) {
    realAccessory = accessory;
  }

  @Override
  public int describeContents() {
    return realAccessory.describeContents();
  }

  @Override
  public String getDescription() {
    return realAccessory.getDescription();
  }

  @Override
  public String getManufacturer() {
    return realAccessory.getManufacturer();
  }

  @Override
  public String getSerial() {
    return realAccessory.getSerial();
  }

  @Override
  public String geUri() {
    return realAccessory.getUri();
  }

  @Override
  public String getVersion() {
    return realAccessory.getVersion();
  }

  @Override
  public UsbAccessory getAccessory() {
    return realAccessory;
  }
}
