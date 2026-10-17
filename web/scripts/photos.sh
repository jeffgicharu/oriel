#!/usr/bin/env bash
# Makes the site's photographs from the masters: each at three widths, in
# AVIF and in WebP for browsers without AVIF, with all metadata removed.
# Usage: web/scripts/photos.sh <masters folder> [name ...]
set -euo pipefail
src="$1"; shift
out="$(cd "$(dirname "$0")/.." && pwd)/static/img/photos"
mkdir -p "$out"
names=("$@")
if [ ${#names[@]} -eq 0 ]; then
  for f in "$src"/*.jpg; do names+=("$(basename "$f" .jpg)"); done
fi
for name in "${names[@]}"; do
  read -r width height < <(magick identify -format '%w %h' "$src/$name.jpg")
  # Portraits never show wider than about 700 px, so they stop at 1400.
  widths=(800 1400)
  [ "$width" -gt "$height" ] && widths+=(2000)
  for w in "${widths[@]}"; do
    magick "$src/$name.jpg" -strip -colorspace sRGB -resize "${w}x>" -quality 52 -define heic:speed=2 "$out/$name-$w.avif"
  done
  # WebP is only for the few browsers without AVIF; two sizes serve them.
  for w in 800 1400; do
    magick "$src/$name.jpg" -strip -colorspace sRGB -resize "${w}x>" -define webp:method=6 -quality 74 "$out/$name-$w.webp"
  done
  rm -f "$out/$name-2000.webp"
  [ "$width" -gt "$height" ] || rm -f "$out/$name-2000.avif"
  printf '%s ' "$name"
done
echo
du -sh "$out"
