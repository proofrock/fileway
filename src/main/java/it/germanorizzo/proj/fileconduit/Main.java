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

import io.javalin.Javalin;
import io.javalin.http.Context;
import io.javalin.http.HttpResponseException;
import io.javalin.http.HttpStatus;

import java.io.IOException;
import java.util.HashSet;
import java.util.Map;
import java.util.Set;
import java.util.concurrent.*;

public class Main {
    private static final Map<String, Conduit> CONDUITS = new ConcurrentHashMap<>();
    private static final Set<String> SECRET_HASHES = new HashSet<>();
    private static final ScheduledExecutorService scheduler = Executors.newSingleThreadScheduledExecutor();

    static {
        scheduler.scheduleAtFixedRate(() -> {
            var cutoffTime = System.currentTimeMillis() - TimeUnit.MINUTES.toMillis(15);
            CONDUITS.entrySet().removeIf(entry -> entry.getValue().getLastAccessed() < cutoffTime);
        }, 1, 1, TimeUnit.MINUTES);
    }

    public static void main(String[] args) {
        System.out.println();
        System.out.println("========================");
        System.out.println("== fileconduit v0.2.0 ==");
        System.out.println("========================");
        System.out.println();

        var env = System.getenv("FILECONDUIT_SECRET_HASHES");
        if (env == null) {
            System.err.println("FATAL: missing environment variable FILECONDUIT_SECRET_HASHES");
            System.exit(1);
        }
        for (var s : env.split(","))
            SECRET_HASHES.add(s.toLowerCase());

        var app = Javalin.create(javalinConfig -> {
            javalinConfig.showJavalinBanner = false;
            javalinConfig.http.disableCompression();
            javalinConfig.http.maxRequestSize = Conduit.CHUNK_SIZE;
        });
        app.get("/dl/{conduitId}", Main::dl);
        app.get("/setup", Main::setup);
        app.get("/ping/{conduitId}", Main::ping);
        app.put("/ul/{conduitId}", Main::ul);

        app.start(8080);
    }

    private static Conduit getConduit(Context ctx) {
        var conduitId = ctx.pathParam("conduitId");
        var conduit = CONDUITS.get(conduitId);
        if (conduit == null) {
            throw new HttpResponseException(404, "Conduit Not Found");
        }
        return conduit;
    }

    public static void dl(Context ctx) throws IOException, InterruptedException {
        var conduit = getConduit(ctx);
        Conduit.Downloadable dlManager;
        try {
            dlManager = conduit.download();
        } catch (IllegalStateException ise) {
            throw new HttpResponseException(410, ise.getMessage());
        }

        ctx.header("Content-Type", "application/octet-stream");
        ctx.header("Content-Disposition", "attachment; filename=\"" + dlManager.getFilename() + "\"");
        ctx.header("Content-Length", Long.toString(dlManager.getSize()));

        try (var os = ctx.outputStream()) {
            int transferred = 0;
            while (true) {
                byte[] bytes = dlManager.getContent().poll(30, TimeUnit.SECONDS);
                if (bytes == null)
                    throw new HttpResponseException(408, "Download timed out. Conduit seems stuck.");

                if (bytes.length == 0) {
                    break;
                }

                os.write(bytes);
                transferred += bytes.length;

                if (transferred >= dlManager.getSize())
                    break;
            }
        } finally {
            CONDUITS.remove(conduit.getConduitId());
        }

        ctx.status(HttpStatus.OK);
    }

    public static void setup(Context ctx) {
        var passedSecret = ctx.header("x-fileconduit-secret");
        if (!SECRET_HASHES.contains(Utils.sha256Hex(passedSecret)))
            throw new HttpResponseException(401, "Secret Mismatch");

        var sizeStr = ctx.queryParam("size");
        var filename = ctx.queryParam("filename");
        if (sizeStr == null || filename == null)
            throw new HttpResponseException(400, "Missing required parameter");

        var size = Long.parseLong(sizeStr);
        var conduit = new Conduit(filename, size, passedSecret);
        CONDUITS.put(conduit.getConduitId(), conduit);
        ctx.result(conduit.getConduitId()).status(HttpStatus.OK);
    }

    // Cleanup of stale conduits

    public static void ping(Context ctx) {
        var conduit = getConduit(ctx);
        if (conduit.isUploadSecretWrong(ctx.header("x-fileconduit-secret")))
            throw new HttpResponseException(401, "Secret Mismatch");

        var ret = "";
        if (conduit.isDownloading())
            ret = Integer.toString(Conduit.CHUNK_SIZE);
        ctx.result(ret);
        ctx.status(HttpStatus.OK);
    }

    public static void ul(Context ctx) throws InterruptedException {
        var conduit = getConduit(ctx);
        if (conduit.isUploadSecretWrong(ctx.header("x-fileconduit-secret")))
            throw new HttpResponseException(401, "Secret Mismatch");

        var content = ctx.bodyAsBytes();
        try {
            conduit.offer(content);
        } catch (TimeoutException te) {
            throw new HttpResponseException(408, te.getMessage());
        }
        ctx.status(HttpStatus.OK);
    }
}