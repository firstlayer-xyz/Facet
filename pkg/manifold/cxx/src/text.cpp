#include "facet_cxx.h"
#include "internal.h"
#include "manifold/cross_section.h"

#include <ft2build.h>
#include FT_FREETYPE_H
#include FT_OUTLINE_H

#include <cstdlib>
#include <cstring>
#include <limits>
#include <cmath>
#include <vector>
#include <string>

using namespace manifold;
using namespace facet_cxx_internal;  // wrap_cs

// Linearize a single FreeType outline contour into a vector of vec2 points.
// Returns false if fewer than 3 points were generated.
static bool linearize_contour(FT_Outline* outline, int contour_start, int contour_end,
                              double scale, double offsetX, int steps,
                              SimplePolygon& out) {
  int n_pts = contour_end - contour_start + 1;
  out.clear();
  out.reserve(n_pts * steps);

  for (int i = 0; i < n_pts; i++) {
    int idx = contour_start + i;
    int tag = FT_CURVE_TAG(outline->tags[idx]);

    if (tag == FT_CURVE_TAG_ON) {
      double x = (double)outline->points[idx].x * scale + offsetX;
      double y = (double)outline->points[idx].y * scale;
      out.push_back({x, y});
    } else if (tag == FT_CURVE_TAG_CONIC) {
      int prev_idx = contour_start + ((i - 1 + n_pts) % n_pts);
      int next_i = (i + 1) % n_pts;
      int next_idx = contour_start + next_i;

      double p0x, p0y;
      if (FT_CURVE_TAG(outline->tags[prev_idx]) == FT_CURVE_TAG_ON) {
        p0x = (double)outline->points[prev_idx].x * scale + offsetX;
        p0y = (double)outline->points[prev_idx].y * scale;
      } else {
        p0x = ((double)outline->points[prev_idx].x + (double)outline->points[idx].x) * 0.5 * scale + offsetX;
        p0y = ((double)outline->points[prev_idx].y + (double)outline->points[idx].y) * 0.5 * scale;
      }

      double cx = (double)outline->points[idx].x * scale + offsetX;
      double cy = (double)outline->points[idx].y * scale;

      double p1x, p1y;
      if (FT_CURVE_TAG(outline->tags[next_idx]) == FT_CURVE_TAG_ON) {
        p1x = (double)outline->points[next_idx].x * scale + offsetX;
        p1y = (double)outline->points[next_idx].y * scale;
      } else {
        p1x = ((double)outline->points[idx].x + (double)outline->points[next_idx].x) * 0.5 * scale + offsetX;
        p1y = ((double)outline->points[idx].y + (double)outline->points[next_idx].y) * 0.5 * scale;
      }

      for (int s = 1; s <= steps; s++) {
        double t = (double)s / (double)steps;
        double mt = 1.0 - t;
        double x = mt*mt*p0x + 2*mt*t*cx + t*t*p1x;
        double y = mt*mt*p0y + 2*mt*t*cy + t*t*p1y;
        out.push_back({x, y});
      }
    } else if (tag == FT_CURVE_TAG_CUBIC) {
      int next_i = (i + 1) % n_pts;
      int next_idx = contour_start + next_i;
      if (FT_CURVE_TAG(outline->tags[next_idx]) != FT_CURVE_TAG_CUBIC) {
        continue;
      }

      int end_i = (i + 2) % n_pts;
      int end_idx = contour_start + end_i;
      int prev_idx = contour_start + ((i - 1 + n_pts) % n_pts);

      double p0x = (double)outline->points[prev_idx].x * scale + offsetX;
      double p0y = (double)outline->points[prev_idx].y * scale;
      double c1x = (double)outline->points[idx].x * scale + offsetX;
      double c1y = (double)outline->points[idx].y * scale;
      double c2x = (double)outline->points[next_idx].x * scale + offsetX;
      double c2y = (double)outline->points[next_idx].y * scale;
      double p1x = (double)outline->points[end_idx].x * scale + offsetX;
      double p1y = (double)outline->points[end_idx].y * scale;

      for (int s = 1; s <= steps; s++) {
        double t = (double)s / (double)steps;
        double mt = 1.0 - t;
        double x = mt*mt*mt*p0x + 3*mt*mt*t*c1x + 3*mt*t*t*c2x + t*t*t*p1x;
        double y = mt*mt*mt*p0y + 3*mt*mt*t*c1y + 3*mt*t*t*c2y + t*t*t*p1y;
        out.push_back({x, y});
      }
      i++; // skip second cubic control point
    }
  }

  return out.size() >= 3;
}

// Compute the horizontal translation that realigns a [0, width] run to
// match the requested halign anchor. "" or "left" → 0 (no shift);
// "center" → -width/2; "right" → -width. Returns NAN on unknown input
// so the caller can surface a hard error instead of silently mis-aligning.
static double halign_offset(const char* halign, double width) {
  if (!halign || halign[0] == '\0') return 0.0;
  if (strcmp(halign, "left") == 0) return 0.0;
  if (strcmp(halign, "center") == 0) return -width * 0.5;
  if (strcmp(halign, "right") == 0) return -width;
  return std::numeric_limits<double>::quiet_NaN();
}

// Compute the vertical translation that realigns a baseline-at-0 run
// (ascender>0, descender<0) to the requested valign anchor. "" or
// "baseline" → 0 (no shift); "top" puts y=0 at ascender top; "bottom"
// puts y=0 at descender bottom; "center" puts y=0 at the (ascender+
// descender)/2 line. NAN on unknown input.
static double valign_offset(const char* valign, double ascender, double descender) {
  if (!valign || valign[0] == '\0') return 0.0;
  if (strcmp(valign, "baseline") == 0) return 0.0;
  if (strcmp(valign, "top") == 0) return -ascender;
  if (strcmp(valign, "bottom") == 0) return -descender;
  if (strcmp(valign, "center") == 0) return -(ascender + descender) * 0.5;
  return std::numeric_limits<double>::quiet_NaN();
}

extern "C" {

void facet_text_to_cross_section(
    const char* font_data, size_t font_len, const char* text, double size_mm,
    const char* halign, const char* valign, FacetSketchRet* out) {

  if (!text || text[0] == '\0') {
    // This early return precedes the main exception barrier below, but
    // `new CrossSection()` can still throw std::bad_alloc. Barrier it here so a
    // failed allocation surfaces a null result instead of unwinding into Go (UB).
    try {
      wrap_cs(new CrossSection(), out);
    } catch (...) {
      out->ptr = nullptr;
      out->size = 0;
    }
    return;
  }

  FT_Library library;
  if (FT_Init_FreeType(&library) != 0) {
    out->ptr = nullptr;
    out->size = 0;
    return;
  }

  // Load from the in-memory font bytes (no filesystem): the default font is
  // embedded on the Go side and a future custom font is supplied as bytes too,
  // so both native and wasm take the same path.
  FT_Face face;
  if (FT_New_Memory_Face(library, (const FT_Byte*)font_data, (FT_Long)font_len, 0, &face) != 0) {
    FT_Done_FreeType(library);
    out->ptr = nullptr;
    out->size = 0;
    return;
  }

  // Exception barrier: from here a throw (FreeType, Polygons/CrossSection
  // allocation, geometry) must release the FreeType handles and surface a null
  // result, never unwind across the extern "C" boundary into Go (UB).
  try {

  double unitsPerEM = (double)face->units_per_EM;
  double scale = size_mm / unitsPerEM;
  double ascender = (double)face->ascender * scale;
  double descender = (double)face->descender * scale;
  int steps = 8;

  // Decode UTF-8 text into Unicode code points
  std::vector<uint32_t> codepoints;
  const unsigned char* p = (const unsigned char*)text;
  while (*p) {
    uint32_t cp;
    if (*p < 0x80) {
      cp = *p++;
    } else if (*p < 0xC0) {
      p++; continue; // invalid continuation byte
    } else if (*p < 0xE0) {
      cp = (*p++ & 0x1F) << 6;
      if (*p) cp |= (*p++ & 0x3F);
    } else if (*p < 0xF0) {
      cp = (*p++ & 0x0F) << 12;
      if (*p) cp |= (*p++ & 0x3F) << 6;
      if (*p) cp |= (*p++ & 0x3F);
    } else {
      cp = (*p++ & 0x07) << 18;
      if (*p) cp |= (*p++ & 0x3F) << 12;
      if (*p) cp |= (*p++ & 0x3F) << 6;
      if (*p) cp |= (*p++ & 0x3F);
    }
    codepoints.push_back(cp);
  }

  Polygons allPolys;
  double advanceX = 0.0;

  for (uint32_t cp : codepoints) {
    FT_UInt glyphIndex = FT_Get_Char_Index(face, cp);
    if (glyphIndex == 0) continue;

    if (FT_Load_Glyph(face, glyphIndex, FT_LOAD_NO_SCALE) != 0) continue;

    FT_Outline* outline = &face->glyph->outline;
    if (outline->n_contours == 0) {
      advanceX += (double)face->glyph->advance.x * scale;
      continue;
    }

    int contour_start = 0;
    for (int c = 0; c < outline->n_contours; c++) {
      int contour_end = outline->contours[c];
      SimplePolygon poly;
      if (linearize_contour(outline, contour_start, contour_end, scale, advanceX, steps, poly)) {
        allPolys.push_back(std::move(poly));
      }
      contour_start = contour_end + 1;
    }

    advanceX += (double)face->glyph->advance.x * scale;
  }

  FT_Done_Face(face); face = nullptr;
  FT_Done_FreeType(library); library = nullptr;

  if (allPolys.empty()) {
    wrap_cs(new CrossSection(), out);
    return;
  }

  double dx = halign_offset(halign, advanceX);
  double dy = valign_offset(valign, ascender, descender);
  if (std::isnan(dx) || std::isnan(dy)) {
    // Unknown halign/valign string — signal failure so the Go side
    // can return a clear error instead of silently mis-aligning.
    out->ptr = nullptr;
    out->size = 0;
    return;
  }

  CrossSection cs(allPolys, CrossSection::FillRule::EvenOdd);
  if (dx != 0.0 || dy != 0.0) {
    cs = cs.Translate({dx, dy});
  }
  wrap_cs(new CrossSection(std::move(cs)), out);

  } catch (...) {
    if (face) FT_Done_Face(face);
    if (library) FT_Done_FreeType(library);
    out->ptr = nullptr;
    out->size = 0;
  }
}

}  // extern "C"
