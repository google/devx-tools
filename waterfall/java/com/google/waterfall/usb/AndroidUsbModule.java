package com.google.waterfall.usb;

import static android.content.Context.USB_SERVICE;

import android.content.Context;
import android.hardware.usb.UsbManager;
import androidx.annotation.VisibleForTesting;
import dagger.Module;
import dagger.Provides;
import javax.inject.Singleton;

/** Dagger module to inject USB manager */
@Module
public class AndroidUsbModule {

  private final UsbService service;
  @VisibleForTesting static UsbManagerIntf usbManager;

  public AndroidUsbModule(UsbService service) {
    this.service = service;
  }

  /**
   * Allow the application context to be injected but require that it be annotated with {@link
   * ForService @Annotation} to explicitly differentiate it from an activity context.
   */
  @Provides
  @Singleton
  @ForService
  Context provideServiceContext() {
    return service;
  }

  @Provides
  @Singleton
  UsbManagerIntf provideUsbManager() {
    if (usbManager != null) {
      return usbManager;
    }
    return new AoaUsbManager((UsbManager) service.getSystemService(USB_SERVICE));
  }
}
