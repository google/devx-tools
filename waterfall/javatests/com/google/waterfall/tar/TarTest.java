package com.google.waterfall.tar;

import static java.nio.charset.StandardCharsets.UTF_8;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.Assert.assertTrue;

import com.google.waterfall.helpers.FileTestHelper;
import java.io.File;
import java.io.FileInputStream;
import java.io.FileOutputStream;
import java.io.IOException;
import java.io.Writer;
import java.nio.file.Files;
import org.junit.Before;
import org.junit.Rule;
import org.junit.Test;
import org.junit.rules.TemporaryFolder;
import org.junit.runner.RunWith;
import org.junit.runners.JUnit4;

@RunWith(JUnit4.class)
public class TarTest {
  private File parentDir;
  private File subDir;
  private File sampleFile;

  @Rule
  public TemporaryFolder rootFolder = new TemporaryFolder();

  @Before
  public void setUp() throws Exception {
    parentDir  = rootFolder.newFolder("parentDir");
    parentDir.mkdir();

    subDir = new File(parentDir, "subDir");
    subDir.mkdir();

    sampleFile = new File(subDir, "sample.txt");
    sampleFile.createNewFile();

    try(Writer writer = Files.newBufferedWriter(sampleFile.toPath(), UTF_8)) {
      writer.write(FileTestHelper.SAMPLE_FILE_CONTENT);
    }
  }

  @Test
  public void testTarFile() throws IOException {
    String fileName = sampleFile.getName();
    String src = sampleFile.getPath();
    String tarDst = rootFolder.getRoot().getPath() + "/" + fileName + ".tar";
    String untarDst = rootFolder.getRoot().getPath() + "/" + fileName;

    try(FileOutputStream fos = new FileOutputStream(new File(tarDst))) {
      Tar.tar(src, fos);
    }

    try(FileInputStream fis = new FileInputStream(new File(tarDst))) {
      Tar.untar(fis, untarDst);
    }

    assertTrue(FileTestHelper.fileContentEquals(untarDst, FileTestHelper.SAMPLE_FILE_CONTENT));
  }

  @Test
  public void testTarDirectory() throws IOException {
    String dirName = subDir.getName();
    String src = subDir.getPath();
    String tarDst = rootFolder.getRoot().getPath() + "/" + dirName+ ".tar";
    String untarDst = rootFolder.getRoot().getPath() + "/" + dirName;
    String untarDstFile = untarDst + "/" + sampleFile.getName();

    try(FileOutputStream fos = new FileOutputStream(new File(tarDst))) {
      Tar.tar(src, fos);
    }

    try(FileInputStream fis = new FileInputStream(new File(tarDst))) {
      Tar.untar(fis, untarDst);
    }

    assertTrue(FileTestHelper.fileContentEquals(untarDstFile, FileTestHelper.SAMPLE_FILE_CONTENT));
  }

  @Test
  public void testTarRecursiveDiretory() throws IOException {
    String src = parentDir.getPath();
    String dstRoot = rootFolder.getRoot().getPath();
    String untarDst = dstRoot + "/" + parentDir.getName();
    String tarDst = untarDst + ".tar";
    String untarDstSubDir = untarDst + "/" + subDir.getName();
    String untarDstFile = untarDstSubDir + "/" + sampleFile.getName();

    try(FileOutputStream fos = new FileOutputStream(new File(tarDst))) {
      Tar.tar(src, fos);
    }

    try(FileInputStream fis = new FileInputStream(new File(tarDst))) {
      Tar.untar(fis, untarDst);
    }

    assertTrue(new File(untarDst).exists());
    assertTrue(new File(untarDstSubDir).exists());
    assertTrue(FileTestHelper.fileContentEquals(untarDstFile, FileTestHelper.SAMPLE_FILE_CONTENT));
  }

  @Test
  public void untarNonexistentThrows() throws IOException {
    String filename = "nonexistent";
    String src = rootFolder.getRoot().getPath() + "/" + filename + ".tar";
    String dst = rootFolder.getRoot().getPath() + "/" + filename;

    try(FileOutputStream fos = new FileOutputStream(new File(dst))) {
      assertThrows(IOException.class, () -> Tar.tar(src, fos));
    }
  }
}
