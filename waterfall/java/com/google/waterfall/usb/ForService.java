package com.google.waterfall.usb;

import static java.lang.annotation.RetentionPolicy.RUNTIME;

import java.lang.annotation.Retention;
import javax.inject.Qualifier;

/** For service annotation */
@Qualifier
@Retention(RUNTIME)
public @interface ForService {}
