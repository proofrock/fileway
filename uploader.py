#
# Copyright (C) 2024- Germano Rizzo
#
# This file is part of fileconduit.
#
# fileconduit is free software: you can redistribute it and/or modify
# it under the terms of the GNU General Public License as published by
# the Free Software Foundation, either version 3 of the License, or
# (at your option) any later version.
#
# fileconduit is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
# GNU General Public License for more details.
#
# You should have received a copy of the GNU General Public License
# along with fileconduit.  If not, see <http://www.gnu.org/licenses/>.
#
import os
import requests
import time

# Secret for uploading
SECRET = "mysecret" # Hashes to 652c7dc687d98c9889304ed2e408c74b611e86a40caa51c4b43f1dd5913c5cd0

# Base URL for all HTTP requests
BASE_URL = "http://localhost:8080"

# Global buffer size for file chunks
# On stable connections, higher is faster
BUFFER_SIZE = 65536

#############################
### Don't touch from here ###
#############################

def upload_file(filepath):
    # Extract filename from path
    filename = os.path.basename(filepath)

    # Get file size
    filesize = os.path.getsize(filepath)

    # Initial call to get conduitId
    init_response = requests.put(
        f"{BASE_URL}/init",
        params={
            "secret": SECRET,
            "filename": filename,
            "size": filesize
        }
    )
    conduitId = int(init_response.text)

    # Output the full conduit URL
    print("== fileconduit v0.0.2 ==")
    print("All set up! Download your file using:")
    print(f"- a browser, from {BASE_URL}/dl/{conduitId}")
    print(f"- a shell, with $> curl -OJ {BASE_URL}/dl/{conduitId}")

    # Initial offset
    current_offset = 0

    # Open file in binary mode
    with open(filepath, 'rb') as file:
        # Poll to check server availability
        while True:
            ping_response = requests.get(f"{BASE_URL}/ping/{conduitId}")
            ping_data = ping_response.json()

            if ping_data['op'] == 1:
                # Server ready, get offset
                server_offset = int(ping_data['arg'])
                break

            # Wait a second before retrying
            time.sleep(1)

        while True:
            # Read chunk to send
            file.seek(current_offset)
            chunk = file.read(BUFFER_SIZE)

            # Send chunk
            ul_response = requests.put(
                f"{BASE_URL}/ul/{conduitId}",
                params={"from": current_offset},
                data=chunk
            )

            # Increment offset
            current_offset += len(chunk)

            # Exit if everything is sent
            if current_offset >= filesize:
                break

# Example usage
if __name__ == "__main__":
    import sys

    if len(sys.argv) < 2:
        print("== fileconduit v0.0.2 ==")
        print("Usage: python uploader.py <file_path>")
        sys.exit(1)

    filepath = sys.argv[1]
    upload_file(filepath)