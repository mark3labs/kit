#!/usr/bin/env node

const { execSync, spawnSync } = require("child_process");
const fs = require("fs");
const path = require("path");
const https = require("https");
const { createWriteStream, mkdirSync, chmodSync, unlinkSync } = fs;

const REPO = "mark3labs/kit";
const BINARY = "kit";

// Get version from package.json
const packageJson = require("./package.json");
const VERSION = packageJson.version;

// Platform mapping
const PLATFORM_MAP = {
  darwin: "darwin",
  linux: "linux",
  win32: "windows",
};

// Arch mapping
const ARCH_MAP = {
  x64: "amd64",
  arm64: "arm64",
};

function getPlatform() {
  const platform = PLATFORM_MAP[process.platform];
  if (!platform) {
    throw new Error(`Unsupported platform: ${process.platform}`);
  }
  return platform;
}

function getArch() {
  const arch = ARCH_MAP[process.arch];
  if (!arch) {
    throw new Error(`Unsupported architecture: ${process.arch}`);
  }
  return arch;
}

function getExtension(platform) {
  return platform === "windows" ? "zip" : "tar.gz";
}

function getBinaryName(platform) {
  // Use different name to avoid conflict with JS wrapper on Unix
  return platform === "windows" ? `${BINARY}.exe` : `${BINARY}-bin`;
}

function download(url, dest) {
  return new Promise((resolve, reject) => {
    const follow = (url, redirects = 0) => {
      if (redirects > 10) {
        reject(new Error("Too many redirects"));
        return;
      }

      https
        .get(url, (response) => {
          if (response.statusCode >= 300 && response.statusCode < 400 && response.headers.location) {
            follow(response.headers.location, redirects + 1);
            return;
          }

          if (response.statusCode !== 200) {
            reject(new Error(`Failed to download: ${response.statusCode}`));
            return;
          }

          const file = createWriteStream(dest);
          response.pipe(file);
          file.on("finish", () => {
            file.close();
            resolve();
          });
          file.on("error", (err) => {
            unlinkSync(dest);
            reject(err);
          });
        })
        .on("error", reject);
    };

    follow(url);
  });
}

function extract(archivePath, destDir, platform) {
  if (platform === "windows") {
    // Use PowerShell to extract zip on Windows
    spawnSync("powershell", [
      "-Command",
      `Expand-Archive -Path "${archivePath}" -DestinationPath "${destDir}" -Force`,
    ]);
  } else {
    // Use tar for Unix systems
    spawnSync("tar", ["-xzf", archivePath, "-C", destDir]);
  }
}

async function main() {
  const platform = getPlatform();
  const arch = getArch();
  const ext = getExtension(platform);
  const binaryName = getBinaryName(platform);

  const filename = `${BINARY}_${VERSION}_${platform}_${arch}.${ext}`;
  const url = `https://github.com/${REPO}/releases/download/v${VERSION}/${filename}`;

  const binDir = path.join(__dirname, "bin");
  const archivePath = path.join(__dirname, filename);
  const binaryPath = path.join(binDir, binaryName);

  console.log(`Installing ${BINARY} v${VERSION} (${platform}/${arch})...`);
  console.log(`Downloading from ${url}...`);

  try {
    // Ensure bin directory exists
    mkdirSync(binDir, { recursive: true });

    // Download archive
    await download(url, archivePath);

    // Extract
    extract(archivePath, binDir, platform);

    // Rename binary (archive contains "kit", we want "kit-bin" on Unix)
    const extractedName = platform === "windows" ? `${BINARY}.exe` : BINARY;
    const extractedPath = path.join(binDir, extractedName);
    if (extractedPath !== binaryPath && fs.existsSync(extractedPath)) {
      fs.renameSync(extractedPath, binaryPath);
    }

    // Make executable on Unix
    if (platform !== "windows") {
      chmodSync(binaryPath, 0o755);
    }

    // Clean up archive
    unlinkSync(archivePath);

    console.log(`Successfully installed ${BINARY} to ${binaryPath}`);
  } catch (err) {
    console.error(`Failed to install ${BINARY}: ${err.message}`);
    process.exit(1);
  }
}

main();
