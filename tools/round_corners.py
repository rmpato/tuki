#!/usr/bin/env python3
"""Round the corners of a screenshot.

A screenshot of a macOS window is a rectangle, but the window isn't: each
corner carries a sliver of whatever was on the desktop behind it. This masks
those corners to transparent so the image sits cleanly on any background —
including GitHub's README, which can't round them with CSS the way the site
can.

Standard library only, because the machine that needs this may not have Pillow.
Input is a BMP (`sips -s format bmp`), output is an RGBA PNG.

    sips -s format bmp shot.png --out shot.bmp
    python3 tools/round_corners.py shot.bmp docs/screenshot.png 14

See runbooks/screenshot.md for the whole procedure.
"""

import struct
import sys
import zlib


def load_bmp(path):
    """Return (width, height, rows) where rows are top-down lists of (r,g,b)."""
    data = open(path, "rb").read()
    pixel_offset = struct.unpack_from("<I", data, 10)[0]
    _, width, height, _, bpp = struct.unpack_from("<IiiHH", data, 14)

    bottom_up = height > 0
    height = abs(height)
    if bpp not in (24, 32):
        raise SystemExit(f"{path}: expected a 24- or 32-bit BMP, got {bpp}-bit")

    stride = ((width * bpp + 31) // 32) * 4
    step = bpp // 8

    rows = []
    for y in range(height):
        source_y = (height - 1 - y) if bottom_up else y
        base = pixel_offset + source_y * stride
        row = []
        for x in range(width):
            i = base + x * step
            row.append((data[i + 2], data[i + 1], data[i]))  # BGR on disk
        rows.append(row)
    return width, height, rows


def alpha_at(x, y, width, height, radius):
    """0 outside the rounded rectangle, 255 inside, antialiased on the arcs."""
    centres = (
        (radius, radius),
        (width - 1 - radius, radius),
        (radius, height - 1 - radius),
        (width - 1 - radius, height - 1 - radius),
    )
    for cx, cy in centres:
        in_x = x < radius if cx == radius else x > width - 1 - radius
        in_y = y < radius if cy == radius else y > height - 1 - radius
        if in_x and in_y:
            distance = ((x - cx) ** 2 + (y - cy) ** 2) ** 0.5
            if distance <= radius - 0.5:
                return 255
            if distance >= radius + 0.5:
                return 0
            return int(255 * (radius + 0.5 - distance))
    return 255


def write_png(path, width, height, rows, radius):
    raw = bytearray()
    for y in range(height):
        raw.append(0)  # filter: None
        for x in range(width):
            r, g, b = rows[y][x]
            raw += bytes((r, g, b, alpha_at(x, y, width, height, radius)))

    def chunk(tag, payload):
        return (
            struct.pack(">I", len(payload))
            + tag
            + payload
            + struct.pack(">I", zlib.crc32(tag + payload) & 0xFFFFFFFF)
        )

    png = (
        b"\x89PNG\r\n\x1a\n"
        + chunk(b"IHDR", struct.pack(">IIBBBBB", width, height, 8, 6, 0, 0, 0))
        + chunk(b"IDAT", zlib.compress(bytes(raw), 9))
        + chunk(b"IEND", b"")
    )
    open(path, "wb").write(png)
    return len(png)


def main():
    if len(sys.argv) not in (3, 4):
        raise SystemExit("usage: round_corners.py <in.bmp> <out.png> [radius]")

    src, dst = sys.argv[1], sys.argv[2]
    radius = int(sys.argv[3]) if len(sys.argv) == 4 else 14

    width, height, rows = load_bmp(src)
    if radius * 2 > min(width, height):
        raise SystemExit(f"radius {radius} is too large for {width}x{height}")

    size = write_png(dst, width, height, rows, radius)
    print(f"{width}x{height} -> {dst} ({size // 1024} KB, radius {radius})")


if __name__ == "__main__":
    main()
