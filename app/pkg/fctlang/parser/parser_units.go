package parser

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// AngleFactors maps angle unit names to their degree conversion factor.
var AngleFactors = map[string]float64{
	"deg":     1,
	"degree":  1,
	"degrees": 1,
	"rad":     180 / math.Pi,
	"radian":  180 / math.Pi,
	"radians": 180 / math.Pi,
	// Surveying / navigation
	"grad":    0.9,            // gradian: 400 grad = 360 deg
	"grads":   0.9,
	"gon":     0.9,            // alias for gradian
	"gons":    0.9,
	"turn":    360,            // full rotation
	"turns":   360,
	"rev":     360,            // alias for turn
	"revs":    360,
	"arcmin":  1.0 / 60,       // arc minute: 1/60 degree
	"arcmins": 1.0 / 60,
	"arcsec":  1.0 / 3600,     // arc second: 1/3600 degree
	"arcsecs": 1.0 / 3600,
	// Military
	"mrad":  180 / (math.Pi * 1000), // milliradian
	"mrads": 180 / (math.Pi * 1000),
	"mil":   360.0 / 6400,           // NATO mil: 6400 mils = 360 deg
	"mils":  360.0 / 6400,
	// Compass / nautical
	"compass_point":  11.25,    // compass point: 32 points = 360 deg
	"compass_points": 11.25,
	"sextant":   60,            // 1/6 of a circle
	"sextants":  60,
	"quadrant":  90,            // 1/4 of a circle
	"quadrants": 90,
}

// UnitFactors maps unit names to their millimeter conversion factor.
// Derived units are computed from base units to ensure runtime float64
// arithmetic matches (e.g. 6 ft == 1 fathom).
// NOTE: must be a package-level var initializer (not init()) so it is ready
// before library.go's init() calls Parse() to parse the stdlib.
var UnitFactors = func() map[string]float64 {
	inch := 25.4
	ft := inch * 12
	yd := ft * 3
	mi := ft * 5280

	return map[string]float64{
		// Metric (full SI prefix ladder)
		"pm":           1e-9,       // picometer
		"picometer":    1e-9,
		"picometers":   1e-9,
		"nm":           1e-6,       // nanometer
		"nanometer":    1e-6,
		"nanometers":   1e-6,
		"um":           0.001,      // micrometer
		"micrometer":   0.001,
		"micrometers":  0.001,
		"mm":           1,          // millimeter
		"millimeter":   1,
		"millimeters":  1,
		"cm":           10,         // centimeter
		"centimeter":   10,
		"centimeters":  10,
		"dm":           100,        // decimeter
		"decimeter":    100,
		"decimeters":   100,
		"m":            1000,       // meter
		"meter":        1000,
		"meters":       1000,
		"dam":          10_000,     // decameter
		"decameter":    10_000,
		"decameters":   10_000,
		"hm":           100_000,    // hectometer
		"hectometer":   100_000,
		"hectometers":  100_000,
		"km":           1_000_000,  // kilometer
		"kilometer":    1_000_000,
		"kilometers":   1_000_000,
		"klick":        1_000_000,
		"klicks":       1_000_000,
		"Mm":           1e9,        // megameter
		"megameter":    1e9,
		"megameters":   1e9,
		"Gm":           1e12,       // gigameter
		"gigameter":    1e12,
		"gigameters":   1e12,
		// Imperial
		"in":     inch,
		"inch":   inch,
		"inches": inch,
		"ft":     ft,
		"foot":   ft,
		"feet":   ft,
		"yd":     yd,
		"yard":   yd,
		"yards":  yd,
		"mi":     mi,
		"mile":   mi,
		"miles":  mi,
		"thou":   inch / 1000,
		// Nautical
		"nmi":            1_852_000, // nautical mile
		"nautical_mile":  1_852_000,
		"nautical_miles": 1_852_000,
		"fathom":         ft * 6,
		"fathoms":        ft * 6,
		// Historical / fun
		"furlong":     ft * 660,
		"furlongs":    ft * 660,
		"chain":       ft * 66,
		"chains":      ft * 66,
		"rod":         ft * 16.5,
		"rods":        ft * 16.5,
		"league":      mi * 3,
		"leagues":     mi * 3,
		"hand":        inch * 4,
		"hands":       inch * 4,
		"cubit":       inch * 18,
		"cubits":      inch * 18,
		"smoot":       inch * 67,  // 5'7"
		"smoots":      inch * 67,
		"barleycorn":  inch / 3,
		"barleycorns": inch / 3,
		// Astronomical
		"parsec":             3.0857e+19,
		"parsecs":            3.0857e+19,
		"ly":                 9.461e+18,
		"light_year":         9.461e+18,
		"light_years":        9.461e+18,
		"au":                 149_597_870_700_000, // astronomical unit (Earth–Sun)
		"astronomical_unit":  149_597_870_700_000,
		"astronomical_units": 149_597_870_700_000,
		// Microscopic
		"micron":    0.001,
		"microns":   0.001,
		"angstrom":  1e-7,
		"angstroms": 1e-7,
		// Sci-fi
		"kellicam":  2_000_000, // Klingon unit of distance, ~2 km (Star Trek III)
		"kellicams": 2_000_000,
		// Grace Hopper's nanosecond — distance light travels in 1 ns
		"hopper":  299.792458, // ~299.8 mm, ~11.8 inches
		"hoppers": 299.792458,
		// Nerdy / absurd
		"potrzebie":     2.263348517438173, // MAD Magazine issue #26 thickness
		"potrzebies":    2.263348517438173,
		"sheppey":       mi * 7.0 / 8.0,   // distance at which sheep remain picturesque
		"sheppeys":      mi * 7.0 / 8.0,
		"beard_second":  5e-6,              // length a beard grows in one second
		"beard_seconds": 5e-6,
		"planck":        1.616255e-32,      // Planck length
		"plancks":       1.616255e-32,
	}
}()

// parseNumberText parses a number token which may be an integer, float, or ratio (e.g. "1/2").
func parseNumberText(text string) (float64, error) {
	if num, denom, ok := strings.Cut(text, "/"); ok {
		n, err := strconv.ParseFloat(num, 64)
		if err != nil {
			return 0, err
		}
		d, err := strconv.ParseFloat(denom, 64)
		if err != nil {
			return 0, err
		}
		if d == 0 {
			return 0, fmt.Errorf("division by zero")
		}
		return n / d, nil
	}
	return strconv.ParseFloat(text, 64)
}
