#!/usr/bin/env python3

#  Copyright 2024 @proofrock
#  Licensed under the Apache License, Version 2.0 (the "License");
#  you may not use this file except in compliance with the License.
#  You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
#  Unless required by applicable law or agreed to in writing, software
#  distributed under the License is distributed on an "AS IS" BASIS,
#  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#  See the License for the specific language governing permissions and
#  limitations under the License.
 
import os

import time
import urllib.request
import urllib.error

# Secret for uploading
SECRET = "mysecret" # Hashes to $2a$10$I.NhoT1acD9XkXmXn1IMSOp0qhZDd63iSw1RfHZP7nzyg/ItX5eVa

# Base URL for all HTTP requests
BASE_URL = "http://localhost:8080"

 ############################
### Don't modify from here ###
 ############################

def upload_file(filepath):
    # Extract filename from path
    filename = os.path.basename(filepath)
    # Get file size
    filesize = os.path.getsize(filepath)

    try:
        # Setup transmission
        setup_url = f"{BASE_URL}/setup?filename={urllib.parse.quote(filename)}&size={filesize}"
        setup_req = urllib.request.Request(setup_url)
        setup_req.add_header("x-fileway-secret", SECRET)
        
        try:
            with urllib.request.urlopen(setup_req, timeout=30) as response:
                if response.status != 200:
                    print("Error in setting up: " + response.read().decode('utf-8'))
                    return
                
                conduitId = response.read().decode('utf-8')

                # Output the full conduit URL
                print("All set up! Download your file using:")
                print(f"- a browser, from {BASE_URL}/dl/{conduitId}")
                print(f"- a shell, with $> curl -OJ {BASE_URL}/dl/{conduitId}")

                # Poll to check server availability and get chunk size
                chunk_size = 0
                while True:
                    ping_url = f"{BASE_URL}/ping/{conduitId}"
                    ping_req = urllib.request.Request(ping_url)
                    ping_req.add_header("x-fileway-secret", SECRET)
                    
                    with urllib.request.urlopen(ping_req, timeout=30) as ping_response:
                        ping_text = ping_response.read().decode('utf-8')
                        if ping_text:
                            chunk_size = int(ping_text)
                            break
                    time.sleep(1)

                # Open file and upload chunks
                with open(filepath, 'rb') as file:
                    while True:
                        chunk = file.read(chunk_size)
                        if len(chunk) == 0:
                            break

                        # Send chunk
                        ul_req = urllib.request.Request(
                            f"{BASE_URL}/ul/{conduitId}", 
                            method='PUT',
                            data=chunk
                        )
                        ul_req.add_header("x-fileway-secret", SECRET)
                        
                        with urllib.request.urlopen(ul_req, timeout=30) as ul_response:
                            if ul_response.status != 200:
                                print("Error in uploading: " + ul_response.read().decode('utf-8'))
                                return

                print("All data sent. Bye!")

        except urllib.error.URLError as e:
            print(f"URL Error: {e}")
    except Exception as e:
        print(f"Unexpected error: {e}")

# Example usage
if __name__ == "__main__":
    import sys

    print("== fileway v0.4.1 ==")
    
    if len(sys.argv) < 2:
        print("Usage: python fileway_ul.py <file_path>")
        sys.exit(1)
    
    # Check if file exists
    if not os.path.exists(sys.argv[1]):
        print(f"Error: File '{sys.argv[1]}' does not exist.")
        sys.exit(1)
    
    # Check if it's a file (not a directory)
    if not os.path.isfile(sys.argv[1]):
        print(f"Error: '{sys.argv[1]}' is not a file.")
        sys.exit(1)

    # Check file readability
    if not os.access(sys.argv[1], os.R_OK):
        print(f"Error: Unable to read file '{sys.argv[1]}'. Check file permissions.")
        sys.exit(1)

    filepath = sys.argv[1]
    upload_file(filepath)