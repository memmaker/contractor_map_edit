#!/bin/zsh

MAPFILE="$1"

head -n 23 "$MAPFILE" | tpaint 80x23 > "$MAPFILE.out"

if [ -s "$MAPFILE.out" ]; then
  {
    cat "$MAPFILE.out"
    tail -n +24 "$MAPFILE"
  } > "$MAPFILE"
else
  rm "$MAPFILE.out"
fi
