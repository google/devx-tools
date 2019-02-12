package com.google.waterfall.helpers;

import java.util.Scanner;
import java.io.File;
import java.io.IOException;

public class FileTestHelper {

  public static boolean fileContentEquals(String filePath, String expected) throws IOException {
    String actual = new Scanner(new File(filePath))
        .useDelimiter("\\A").next();
    return actual.equals(expected);
  }

  public static final String SAMPLE_FILE_CONTENT =
      "Lorem ipsum dolor sit amet, consectetur adipiscing "
          + "elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad "
          + "minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo "
          + "consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum "
          + "dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt "
          + "in culpa qui officia deserunt mollit anim id est laborum.";
}