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

#############################
### Don't touch from here ###
#############################

def upload_file(filepath):
    # Extract filename from path
    filename = os.path.basename(filepath)
    # Get file size
    filesize = os.path.getsize(filepath)

    # Open file in binary mode
    with open(filepath, 'rb') as file: # I open it here to catch errors early
        # Setup transmission
        init_response = requests.get(
            f"{BASE_URL}/setup",
            params={
                "filename": filename,
                "size": filesize
            },
            headers={ "x-fileconduit-secret": SECRET },
            timeout=30
        )
        conduitId = init_response.text

        # Output the full conduit URL
        print("== fileconduit v0.2.0 ==")
        print("All set up! Download your file using:")
        print(f"- a browser, from {BASE_URL}/dl/{conduitId}")
        print(f"- a shell, with $> curl -OJ {BASE_URL}/dl/{conduitId}")

        # Poll to check server availability and get chunk size
        chunk_size = 0
        while True:
            ping_response = requests.get(
                f"{BASE_URL}/ping/{conduitId}",
                headers={ "x-fileconduit-secret": SECRET },
                timeout=30)
            if ping_response.text != "":
                chunk_size = int(ping_response.text)
                break
            time.sleep(1) # Cycle again waiting 1s

        while True:
            chunk = file.read(chunk_size)
            if len(chunk) == 0:
                break

            # Send chunk
            ul_response = requests.put(
                f"{BASE_URL}/ul/{conduitId}",
                headers={ "x-fileconduit-secret": SECRET },
                data=chunk,
                timeout=30
            )

            ul_response.raise_for_status()

# Example usage
if __name__ == "__main__":
    import sys

    if len(sys.argv) < 2:
        print("== fileconduit v0.2.0 ==")
        print("Usage: python uploader.py <file_path>")
        sys.exit(1)

    filepath = sys.argv[1]
    upload_file(filepath)