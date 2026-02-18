#!/bin/bash

# Exit immediately if a command exits with a non-zero status.
set -e

# Define variables
INSTALL_DIR="/usr/local/bin"
SERVICE_FILE="/etc/systemd/system/alsamixer-web.service"
USER="alsamixer"
GROUP="alsamixer"
BINARY_NAME="alsamixer-web"

# Check for root/sudo privileges
if [ "$EUID" -ne 0 ]; then
  echo "Please run this script with sudo or as root."
  exit 1
fi

# Create user and group if they don't exist
if ! getent group $GROUP > /dev/null; then
  echo "Creating group $GROUP..."
  groupadd $GROUP
fi

if ! getent passwd $USER > /dev/null; then
  echo "Creating user $USER..."
  useradd -r -g $GROUP -s /sbin/nologin $USER
fi

# Copy binary to installation directory
echo "Copying $BINARY_NAME to $INSTALL_DIR..."
cp $BINARY_NAME $INSTALL_DIR/

# Set executable permissions
echo "Setting executable permissions for $INSTALL_DIR/$BINARY_NAME..."
chmod +x $INSTALL_DIR/$BINARY_NAME

# Copy service file
echo "Copying service file to $SERVICE_FILE..."
cp deploy/alsamixer-web.service $SERVICE_FILE

# Reload systemd daemon
echo "Reloading systemd daemon..."
systemctl daemon-reload

# Enable and start the service
echo "Enabling and starting alsamixer-web service..."
systemctl enable alsamixer-web.service
systemctl start alsamixer-web.service

# Print status message
echo "Installation complete. Service 'alsamixer-web' is running."
systemctl status alsamixer-web.service