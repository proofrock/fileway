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

import it.germanorizzo.proj.fileconduit.internals.Chunk;
import it.germanorizzo.proj.fileconduit.internals.Command;
import it.germanorizzo.proj.fileconduit.internals.Downloadable;

import java.io.IOException;
import java.security.SecureRandom;
import java.util.concurrent.BlockingQueue;
import java.util.concurrent.LinkedBlockingQueue;

public class Conduit {
    private static final SecureRandom RND = new SecureRandom();

    private final long conduitId;
    private final String filename;
    private final long size;

    private long lastAccessed = System.currentTimeMillis();
    private long currentPosition = -1;
    private BlockingQueue<Chunk> downloadStream = null;

    public Conduit(String filename, long size) {
        this.conduitId = Math.abs(RND.nextLong());
        this.filename = filename;
        this.size = size;
    }

    private void touch() {
        lastAccessed = System.currentTimeMillis();
    }

    public long getConduitId() {
        return conduitId;
    }

    public long getLastAccessed() {
        return lastAccessed;
    }

    private synchronized boolean isDownloading() {
        return downloadStream != null || currentPosition >= 0;
    }

    public synchronized Downloadable download() {
        if (isDownloading())
            throw new IllegalStateException("Can't download when there is already a Downloadable");

        touch();
        currentPosition = 0;
        downloadStream = new LinkedBlockingQueue<>();
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
            public BlockingQueue<Chunk> getContent() {
                return downloadStream;
            }
        };
    }

    public synchronized Command ping() {
        touch();
        if (isDownloading())
            return new Command(1, currentPosition);
        return new Command(0, 0);
    }

    public synchronized void offer(long from, byte[] content) throws IOException {
        touch();
        if (currentPosition != from)
            throw new IOException("Wrong position, I asked for " + currentPosition);
        if (content.length > 0) {
            downloadStream.add(new Chunk(false, content));
            currentPosition += content.length;
            if (currentPosition >= size)
                downloadStream.add(new Chunk(true, new byte[]{}));
        } else {
            downloadStream.add(new Chunk(true, content));
        }
    }
}
