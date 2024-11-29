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

import java.util.Objects;
import java.util.concurrent.ArrayBlockingQueue;
import java.util.concurrent.BlockingQueue;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.TimeoutException;
import java.util.concurrent.atomic.AtomicBoolean;
import java.util.concurrent.atomic.AtomicLong;

public class Conduit {
    public static final int CHUNK_SIZE = 16 * 1024 * 1024; // 16Mb
    private static final int IDS_LENGTH = 33; // 192 bit
    private static final int CHUNK_QUEUE_SIZE = 8; // 128Mb
    private final String secret;
    private final String conduitId;
    private final String filename;
    private final long size;
    private final BlockingQueue<byte[]> chunkQueue = new ArrayBlockingQueue<>(CHUNK_QUEUE_SIZE);
    private final AtomicLong lastAccessed = new AtomicLong(System.currentTimeMillis());
    private final AtomicBoolean downloadStarted = new AtomicBoolean(false);
    public Conduit(String filename, long size, String secret) {
        this.conduitId = Utils.genRandomString(IDS_LENGTH);
        this.filename = filename;
        this.size = size;
        this.secret = secret;
    }

    public String getConduitId() {
        return conduitId;
    }

    public boolean isUploadSecretWrong(String candidate) {
        return !Objects.equals(secret, candidate);
    }

    public long getLastAccessed() {
        return lastAccessed.get();
    }

    private void touch() {
        lastAccessed.set(System.currentTimeMillis());
    }

    public Downloadable download() throws InterruptedException {
        if (downloadStarted.get())
            throw new IllegalStateException("Conduit Already Downloading or Downloaded");

        touch();
        downloadStarted.set(true);

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
            public BlockingQueue<byte[]> getContent() {
                return chunkQueue;
            }
        };
    }

    public boolean isDownloading() {
        touch();
        return downloadStarted.get();
    }

    public void offer(byte[] content) throws TimeoutException, InterruptedException {
        touch();
        var timeout = !chunkQueue.offer(content, 30, TimeUnit.SECONDS);
        if (timeout) {
            throw new TimeoutException("Upload timed out. Conduit seems stuck.");
        }
    }

    public interface Downloadable {
        String getFilename();

        long getSize();

        BlockingQueue<byte[]> getContent();
    }
}
