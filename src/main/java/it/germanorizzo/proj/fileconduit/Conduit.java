/*
 * Copyright (C) 2024- Germano Rizzo
 *
 * This file is part of fileconduit.
 *
 * fileconduit is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * fileconduit is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with fileconduit.  If not, see <http://www.gnu.org/licenses/>.
 */
package it.germanorizzo.proj.fileconduit;

import java.io.InputStream;
import java.security.SecureRandom;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.atomic.AtomicBoolean;
import java.util.concurrent.atomic.AtomicLong;
import java.util.concurrent.atomic.AtomicReference;

public class Conduit {
    private static final SecureRandom RND = new SecureRandom();

    private final long conduitId;
    private final String filename;
    private final long size;

    private final AtomicLong lastAccessed = new AtomicLong(System.currentTimeMillis());
    private final AtomicBoolean downloading = new AtomicBoolean(false);
    private final AtomicReference<InputStream> downloadStream = new AtomicReference<>();
    private final CountDownLatch senderReady = new CountDownLatch(1);
    private final CountDownLatch receivedDone = new CountDownLatch(1);

    public Conduit(String filename, long size) {
        this.conduitId = Math.abs(RND.nextLong());
        this.filename = filename;
        this.size = size;
    }

    public long getConduitId() {
        return conduitId;
    }

    public long getLastAccessed() {
        return lastAccessed.get();
    }

    private void touch() {
        lastAccessed.set(System.currentTimeMillis());
    }

    public Downloadable download() throws InterruptedException {
        if (downloading.get())
            throw new IllegalStateException("Can't download when there is already a Downloadable");

        touch();
        downloading.set(true);
        senderReady.await();

        return new Downloadable() {
            @Override
            public String getFilename() {
                return filename;
            }

            @Override
            public long getSize() {
                return size;
            }

            @Override
            public InputStream getContent() {
                return downloadStream.get();
            }
        };
    }

    public void doneReceiving() {
        receivedDone.countDown();
    }

    public int isDownloading() {
        touch();
        return downloading.get() ? 1: 0;
    }

    public void offer(InputStream content) throws InterruptedException {
        touch();
        downloadStream.set(content);
        senderReady.countDown();
        receivedDone.await();
    }
}
