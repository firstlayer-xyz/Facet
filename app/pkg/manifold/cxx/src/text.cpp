#include "facet_cxx.h"
#include "manifold/cross_section.h"

#include <ft2build.h>
#include FT_FREETYPE_H
#include FT_OUTLINE_H

#include <cstdlib>
#include <cstring>
#include <vector>
#include <string>

using namespace manifold;

static CrossSection* as_cpp_cs(ManifoldCrossSection* cs) {
  return reinterpret_cast<CrossSection*>(cs);
}
static ManifoldCrossSection* as_c_cs(CrossSection* cs) {
  return reinterpret_cast<ManifoldCrossSection*>(cs);
}

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

extern "C" {

ManifoldCrossSection* facet_text_to_cross_section(
    const char* font_path, const char* text, double size_mm) {

  if (!text || text[0] == '\0') {
    return as_c_cs(new CrossSection());
  }

  FT_Library library;
  if (FT_Init_FreeType(&library) != 0) {
    return nullptr;
  }

  FT_Face face;
  if (FT_New_Face(library, font_path, 0, &face) != 0) {
    FT_Done_FreeType(library);
    return nullptr;
  }

  double unitsPerEM = (double)face->units_per_EM;
  double scale = size_mm / unitsPerEM;
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

  FT_Done_Face(face);
  FT_Done_FreeType(library);

  if (allPolys.empty()) {
    return as_c_cs(new CrossSection());
  }

  return as_c_cs(new CrossSection(CrossSection(allPolys, CrossSection::FillRule::EvenOdd)));
}

}  // extern "C"
