package com.google.waterfall.usb;

import android.app.Activity;
import android.content.Intent;
import android.hardware.usb.UsbAccessory;
import android.hardware.usb.UsbManager;
import android.os.Bundle;
import android.util.Log;

/* UsbActivity serves as an entry point to the UsbService.
 * Android instantiates this activity when an USB accessory
 * is attached. This actity in turn instantiates the UBS service.
 */
public class UsbActivity extends Activity {

  public static final String TAG = "Waterfall.UsbActivity";
  public static final String USB_SERVICE = "com.google.waterfall.usb.USB_SERVICE";

  @Override
  protected void onCreate(Bundle savedInstanceState) {
    super.onCreate(savedInstanceState);
    Intent intent = getIntent();
    if (intent != null) {
      handleIntent(intent);
    }
  }

  @Override
  protected void onResume() {
    super.onResume();

    // Needed to work with NoDisplay theme.
    finish();
  }

  @Override
  protected void onNewIntent(Intent intent) {
    handleIntent(intent);
  }

  private void handleIntent(Intent intent) {
    // Handle accessory attached events. Ignore everything else.
    if (UsbManager.ACTION_USB_ACCESSORY_ATTACHED.equals(intent.getAction())) {
      Log.d(TAG, "Received ACTION_USB_ACCESSORY_ATTACHED intent. Starting USB service.");
      UsbAccessory accessory = (UsbAccessory) intent.getParcelableExtra(UsbManager.EXTRA_ACCESSORY);
      Intent serviceIntent = new Intent(this, UsbService.class);
      serviceIntent.putExtra(UsbManager.EXTRA_ACCESSORY, accessory);
      startService(serviceIntent);
    }
  }
}
