include <BOSL2/std.scad>

// A hollow ring and a hollow bushing side by side.
tube(h=6, od=20, id=14);
right(30) tube(h=10, or=5, ir=3);
