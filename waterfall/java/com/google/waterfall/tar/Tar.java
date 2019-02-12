package com.google.waterfall.tar;

import com.google.common.io.ByteStreams;
import java.io.File;
import java.io.FileOutputStream;
import java.io.IOException;
import java.io.InputStream;
import java.io.OutputStream;
import java.nio.file.FileVisitResult;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.nio.file.SimpleFileVisitor;
import java.nio.file.attribute.BasicFileAttributes;
import org.apache.commons.compress.archivers.tar.TarArchiveEntry;
import org.apache.commons.compress.archivers.tar.TarArchiveInputStream;
import org.apache.commons.compress.archivers.tar.TarArchiveOutputStream;

/** Package tar implements utilities to tar/untar a directory tree */
public final class Tar {

  private Tar() {}

  /**
   * Tar serializes a directory or a file into the provided output stream.
   * @param src Absolute or relative path to file or directory. If directory is provided, the
   * output will be organized under one directory.
   * @param output
   * @throws IOException
   */
  public static void tar(String src, OutputStream output) throws IOException {
    Path base = Paths.get(src);
    TarArchiveOutputStream tarOutput = new TarArchiveOutputStream(output);
    tarOutput.setLongFileMode(TarArchiveOutputStream.LONGFILE_POSIX);

    Files.walkFileTree(
        base,
        new SimpleFileVisitor<Path>() {
          @Override
          public FileVisitResult visitFile(Path path, BasicFileAttributes attrs)
              throws IOException {
            addFileToTar(tarOutput, base, path);
            return FileVisitResult.CONTINUE;
          }

          @Override
          public FileVisitResult preVisitDirectory(Path filePath, BasicFileAttributes attrs)
              throws IOException {
            addFileToTar(tarOutput, base, filePath);
            return FileVisitResult.CONTINUE;
          }
        });

    tarOutput.finish();
  }

  private static void addFileToTar(TarArchiveOutputStream tarOutput, Path base, Path filePath)
      throws IOException {

    if (Files.isDirectory(filePath)) {
      tarOutput.putArchiveEntry(
          new TarArchiveEntry(
            filePath.toFile(),
            base.relativize(filePath).toString()));
    } else if (Files.isSymbolicLink(filePath)) {
      Path target = filePath.toRealPath();

      if (target.startsWith(base)) {
        String linkName = getRelativeSymLinkTarget(filePath).toString();
        TarArchiveEntry entry =
            new TarArchiveEntry(
                base.relativize(filePath).toString(),
                TarArchiveEntry.LF_SYMLINK);

        entry.setLinkName(linkName);
        tarOutput.putArchiveEntry(entry);
      } else if (Files.isRegularFile(target) && target.toFile().exists()) {
        writeRegularFile(tarOutput, target, base.relativize(filePath).toString());
      }
    } else if(Files.isRegularFile(filePath)) {
      writeRegularFile(tarOutput, filePath, base.relativize(filePath).toString());
    } else {
      return;
    }

    tarOutput.closeArchiveEntry();
  }

  private static void writeRegularFile(TarArchiveOutputStream tos, Path path, String name)
      throws IOException {
    byte[] contents = Files.readAllBytes(path);
    TarArchiveEntry entry = new TarArchiveEntry(path.toFile(), name);
    entry.setSize(contents.length);
    tos.putArchiveEntry(entry);
    tos.write(contents);
  }

  private static Path getRelativeSymLinkTarget(Path sourcePath) throws IOException {
    Path parent = sourcePath.getParent();
    Path resolvedLinkTarget = parent.resolve(Files.readSymbolicLink(sourcePath));
    return parent.relativize(resolvedLinkTarget).normalize();
  }

  /**
   * Untar a tar input stream to the provided output file or directory.
   * @param input An input stream that provides data that is serialized as a tar.
   * @param dst Absolute or relative path to file or directory. If directory is provided, the
   * output will be organized under that directory. If it is a file, the tar source must be a file
   * and will replace whatever is at that destination.
   * @throws IOException
   */
  public static void untar(InputStream input, String dst) throws IOException {
    TarArchiveInputStream tarIn = new TarArchiveInputStream(input);
    TarArchiveEntry tarEntry = tarIn.getNextTarEntry();

    OutputLocation  outputLocation = determineOutputLocations(dst, tarEntry);
    String rootReplace = outputLocation.rootReplacement;
    Path outputPath = outputLocation.outputPath;
    File outputRootDir = outputLocation.outputRootDir;

    if (!outputRootDir.exists() && !outputRootDir.mkdirs()) {
      throw new IOException(
          String.format("Failed to create directory %s.", outputRootDir.toPath()));
    }

    while (true) {
      if (tarEntry.isSymbolicLink()) {
        Files.createSymbolicLink(outputPath, Paths.get(tarEntry.getLinkName()));
      } else if (tarEntry.isFile()) {
        if (outputPath.toFile().exists()) {
          recursiveDelete(outputPath);
        }

        outputPath.toFile().createNewFile();
        try (OutputStream fos = new FileOutputStream(outputPath.toFile(), false)) {
          ByteStreams.copy(tarIn, fos);
        }
      } else if (tarEntry.isDirectory()) {
        if (!outputPath.toFile().exists() && !outputPath.toFile().mkdirs()) {
          throw new IOException(String.format("Failed to create directory %s.", outputPath));
        }
      } else {
        throw new RuntimeException("Unsupported tar entry: " + tarEntry);
      }

      tarEntry = tarIn.getNextTarEntry();
      if (tarEntry == null) {
        break;
      }

      int idx = tarEntry.getName().indexOf(rootReplace);
      if (idx >= 0) {
        outputPath = Paths.get(outputRootDir.toString(), tarEntry.getName().substring(idx + rootReplace.length()));
      } else {
        outputPath = Paths.get(outputRootDir.toString(), tarEntry.getName());
      }
    }

    tarIn.close();
  }

  private static OutputLocation determineOutputLocations(String dst, TarArchiveEntry tarEntry) {
    File dstFile = new File(dst);
    String rootReplacement = "";
    Path outputPath;
    File outputRootDir;

    // determine what the outputRootFolder and first output path are
    if(dstFile.exists()) {
      if (dstFile.isDirectory()) {
        outputRootDir = dstFile;
        outputPath = Paths.get(dst, tarEntry.getName());
      } else if(tarEntry.isFile()) {
        outputRootDir = dstFile.getParentFile();
        outputPath = Paths.get(dst);
      } else {
        throw new RuntimeException(String.format(
            "Tar source is a directory and destination file already exists '%s'", dst));
      }
    } else {
      if(tarEntry.isDirectory()) {
        outputRootDir = dstFile;
        rootReplacement = tarEntry.getName();
      } else {
        outputRootDir = dstFile.getParentFile();
      }

      outputPath = Paths.get(dst);
    }

    return new OutputLocation(outputPath, outputRootDir, rootReplacement);
  }

  private static void recursiveDelete(Path path) throws IOException {
    Files.walkFileTree(path, new SimpleFileVisitor<Path>() {
      @Override
      public FileVisitResult visitFile(Path file, BasicFileAttributes attrs) throws IOException {
        Files.delete(file);
        return FileVisitResult.CONTINUE;
      }

      @Override
      public FileVisitResult postVisitDirectory(Path dir, IOException exc) throws IOException {
        Files.delete(dir);
        return FileVisitResult.CONTINUE;
      }
    });
  }

  private static class OutputLocation {
    public Path outputPath;
    public File outputRootDir;
    public String rootReplacement;

    public OutputLocation(Path output, File root, String rootReplace) {
      this.outputPath = output;
      this.outputRootDir = root;
      this.rootReplacement = rootReplace;
    }
  }
}
