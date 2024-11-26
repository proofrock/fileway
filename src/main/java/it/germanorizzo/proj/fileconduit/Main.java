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
import it.germanorizzo.proj.fileconduit.internals.Downloadable;

import java.io.IOException;
import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.Executors;
import java.util.concurrent.ScheduledExecutorService;
import java.util.concurrent.TimeUnit;

public class Main {
    private static final Map<Long, Conduit> conduits = new ConcurrentHashMap<>();

    public static void main(String[] args) {
        var app = Javalin.create(javalinConfig -> {
            javalinConfig.showJavalinBanner = false;
            javalinConfig.http.disableCompression();
        }).get("/dl/{conduitId}", Main::dl).put("/init", Main::init).get("/ping/{conduitId}", Main::ping).put("/ul/{conduitId}", Main::ul).start(8080);
    }

    private static Conduit getConduit(Context ctx) {
        var conduitId = Long.parseLong(ctx.pathParam("conduitId"));
        var conduit = conduits.get(Long.valueOf(conduitId));
        if (conduit == null) {
            throw new HttpResponseException(404, "Conduit Not Found");
        }
        return conduit;
    }

    public static Context dl(Context ctx) throws IOException, InterruptedException {
        var conduit = getConduit(ctx);
        Downloadable dlManager;
        try {
            dlManager = conduit.download();
        } catch (IllegalStateException ise) {
            throw new HttpResponseException(410, "Conduit Already Downloading or Downloaded");
        }

        ctx.header("Content-Type", "application/octet-stream");
        ctx.header("Content-Disposition", "attachment; filename=\"" + dlManager.getFilename() + "\"");
        ctx.header("Content-Length", Long.toString(dlManager.getSize()));
        try (var os = ctx.outputStream()) {
            while (true) {
                var chunk = dlManager.getContent().take();
                if (chunk.finished()) {
                    os.flush();
                    break;
                }
                os.write(chunk.chunk());
            }
        }
        conduits.remove(Long.valueOf(conduit.getConduitId()));
        return ctx.status(200);
    }

    public static Context init(Context ctx) {
        var secret = System.getenv("FILECONDUIT_SECRET_HASH");
        var passedSecret = ctx.queryParam("secret");
        if (!Utils.equalsIgnoreCase(secret, Utils.sha256Hex(passedSecret)))
            throw new HttpResponseException(401, "Secret Mismatch");

        var filename = ctx.queryParam("filename");
        var size = Long.parseLong(ctx.queryParam("size"));
        var conduit = new Conduit(filename, size);
        conduits.put(Long.valueOf(conduit.getConduitId()), conduit);
        return ctx.result(Long.toString(conduit.getConduitId())).status(200);
    }

    public static Context ping(Context ctx) {
        return ctx.json(getConduit(ctx).ping());
    }

    public static Context ul(Context ctx) throws IOException {
        var conduit = getConduit(ctx);
        var from = Long.parseLong(ctx.queryParam("from"));
        var content = ctx.bodyAsBytes();
        conduit.offer(from, content);
        return ctx.status(200);
    }

    // Cleanup of stale conduits

    private static final ScheduledExecutorService scheduler = Executors.newSingleThreadScheduledExecutor();

    static {
        scheduler.scheduleAtFixedRate(() -> {
            long cutoffTime = System.currentTimeMillis() - TimeUnit.MINUTES.toMillis(15);
            conduits.entrySet().removeIf(entry -> entry.getValue().getLastAccessed() < cutoffTime);
        }, 1, 1, TimeUnit.MINUTES);
    }
}