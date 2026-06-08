include <BOSL2/std.scad>

// A tilted bar, a bolt-circle of pins, and a hollow rectangular frame.
rot([0, 0, 30]) cuboid([20, 4, 4]);
zrot_copies(n=5, r=15) cyl(h=8, r=1.5);
up(20) rect_tube(h=6, size=[24, 18], wall=3);
