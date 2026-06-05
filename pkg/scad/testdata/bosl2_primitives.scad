include <BOSL2/std.scad>

// A centered box lifted up, with a cone-free cylinder beside it.
up(6) cuboid([20, 20, 12]);
left(20) cyl(h=15, d=8);
zrot(45) cuboid(10);
