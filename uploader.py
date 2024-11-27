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
    print("== fileconduit v0.1.0 ==")
    print("All set up! Download your file using:")
    print(f"- a browser, from {BASE_URL}/dl/{conduitId}")
    print(f"- a shell, with $> curl -OJ {BASE_URL}/dl/{conduitId}")

    # Initial offset
    current_offset = 0

    # Poll to check server availability
    while True:
        ping_response = requests.get(f"{BASE_URL}/ping/{conduitId}")
        if ping_response.text == '1':
            break
        time.sleep(1) # Cycle again waiting 1s


    # Open file in binary mode
    with open(filepath, 'rb') as file:
        response = requests.put(
            f"{BASE_URL}/ul/{conduitId}",
            data=file,  # Directly stream file contents
        )

        # Raise an exception for HTTP errors
        response.raise_for_status()

# Example usage
if __name__ == "__main__":
    import sys

    if len(sys.argv) < 2:
        print("== fileconduit v0.1.0 ==")
        print("Usage: python uploader.py <file_path>")
        sys.exit(1)

    filepath = sys.argv[1]
    upload_file(filepath)