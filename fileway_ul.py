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
 
# Base URL for all HTTP requests
BASE_URL = "http://localhost:8080"

# Secret for uploading
# If this row is commented out, it will ask for it at runtime
SECRET = "mysecret" # Hashes to $2a$10$I.NhoT1acD9XkXmXn1IMSOp0qhZDd63iSw1RfHZP7nzyg/ItX5eVa

 ############################
### Don't modify from here ###
 ############################

import os
import time
import urllib.request
import urllib.error
import json
import zipfile
import tempfile
import random
import string
import atexit

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
                chunk_plan = []
                while True:
                    ping_url = f"{BASE_URL}/ping/{conduitId}"
                    ping_req = urllib.request.Request(ping_url)
                    ping_req.add_header("x-fileway-secret", SECRET)
                    
                    with urllib.request.urlopen(ping_req, timeout=30) as ping_response:
                        ping_text = ping_response.read()
                        if ping_text:
                            chunk_plan = json.loads(ping_text)
                            if len(chunk_plan) > 0:
                                break

                    time.sleep(1)

                # Open file and upload chunks
                with open(filepath, 'rb') as file:
                    print("", end="\r")
                    for lap, chunk_size in enumerate(chunk_plan):
                        perc = round(lap*100/len(chunk_plan), 1)
                        print(f"Uploading chunk {lap+1}/{len(chunk_plan)}: {perc}%", end="\r")

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

                print("All data sent. Bye!                     ")

        except urllib.error.URLError as e:
            print(f"URL Error: {e}")
    except Exception as e:
        print(f"Unexpected error: {e}")

def create_temp_zip(paths_list):
    try:
        random_string = ''.join(random.choices(string.ascii_letters + string.digits, k=4))
        zip_filename = f"fileway_{random_string}.zip"
        
        temp_dir = tempfile.gettempdir()
        zip_path = os.path.join(temp_dir, zip_filename)
        
        atexit.register(lambda: os.remove(zip_path) if os.path.exists(zip_path) else None)
        
        with zipfile.ZipFile(zip_path, 'w', zipfile.ZIP_DEFLATED) as zipf:
            for path in paths_list:
                if os.path.exists(path):
                    if os.path.isfile(path):
                        zipf.write(path, os.path.basename(path))
                    elif os.path.isdir(path):
                        for root, _, files in os.walk(path):
                            for file in files:
                                file_path = os.path.join(root, file)
                                arcname = os.path.relpath(file_path, os.path.dirname(path))
                                zipf.write(file_path, arcname)
                else:
                    print(f"Error: Path not found: {path}")
                    return None
        
        return zip_path
    
    except Exception as e:
        print(f"Error creating ZIP file: {str(e)}")
        return None

def abort_usage():
    print("Usage: python3 fileway_ul.py [--zip] <file_path1> [<file_path_2>] [...]")
    print(" (if --zip is specified, more than one file or dir can be provided)")
    sys.exit(1)

# Example usage
if __name__ == "__main__":
    import sys

    print("== fileway v0.6.1 ==")
    print()
    
    try:
        SECRET
    except NameError:
        print()
        from getpass import getpass
        SECRET = getpass('Please enter secret: ')
        print()
    
    is_zip = False
    try:
        is_zip = sys.argv[1] == '--zip'
    except IndexError:
        abort_usage()
    
    file = ""
    if is_zip:
        if len(sys.argv) < 3:
            abort_usage()
        print("Zipping files...")
        file = create_temp_zip(sys.argv[2:])
        if file == None:
            sys.exit(1)
        print(f"Created upload file '{file}'")
    else:
        if len(sys.argv) != 2:
            abort_usage()
        file = sys.argv[1]
    
    # Check if file exists
    if not os.path.exists(file):
        print(f"Error: File '{file}' does not exist.")
        sys.exit(1)
    
    # Check if it's a file (not a directory)
    if not os.path.isfile(file):
        print(f"Error: '{file}' is not a file.")
        sys.exit(1)

    # Check file readability
    if not os.access(file, os.R_OK):
        print(f"Error: Unable to read file '{file}'. Check file permissions.")
        sys.exit(1)

    upload_file(file)
