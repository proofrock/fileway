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

import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;

public class Utils {
    private Utils() {
    }

    public static boolean equalsIgnoreCase(String a, String b) {
        if (a == null) {
            return b == null;
        }
        return a.equalsIgnoreCase(b);
    }

    public static String sha256Hex(String input) {
        MessageDigest md;
        try {
            md = MessageDigest.getInstance("SHA-256");
        } catch (NoSuchAlgorithmException e) {
            throw new RuntimeException(e);
        }
        var hash = md.digest(input.getBytes());
        var hexString = new StringBuilder();
        for (var b : hash) {
            hexString.append(String.format("%02x", Byte.valueOf(b)));
        }
        return hexString.toString();
    }
}
