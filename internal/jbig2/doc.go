// Copyright 2012 Mozilla Foundation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package jbig2 decodes JBIG2 images as embedded in PDF files
// (the JBIG2Decode filter: an optional JBIG2Globals stream plus the
// page stream, both without the JBIG2 file header).
//
// Ported from pdf.js (Mozilla Foundation), Apache License 2.0:
// src/core/jbig2.js, src/core/arithmetic_decoder.js and src/core/ccitt.js
// (the latter itself a port of XPDF's decoder, Glyph & Cog, LLC).
// See the NOTICE file in this directory.
//
// The port follows pdf.js closely and adds most of what pdf.js lacks:
//
//   - as in pdf.js: generic regions (MMR and arithmetic, templates 0-3
//     with adaptive pixels, typical prediction TPGDON), symbol
//     dictionaries and text regions (arithmetic and Huffman, refinement
//     and aggregation), pattern dictionaries and halftone regions,
//     custom and standard Huffman tables, page information;
//   - not in pdf.js: generic refinement regions (of the page or of an
//     intermediate region), intermediate text, halftone and generic
//     regions, typical prediction in refinement (TPGRON), refinement with
//     Huffman coding in symbol dictionaries and text regions, retained
//     bitmap coding contexts of symbol dictionaries, halftone skipping and
//     combination operators, the AND, XNOR and REPLACE operators, pages of
//     unknown height (end of stripe segments) and generic regions of
//     unknown length.
//
// Not supported (an error is returned): the extended generic region
// template (EXTTEMPLATE) and the colour extension of the 2018 edition of
// T.88. Standalone multi-page files are only used in tests.
//
// pdf.js bugs fixed: symbol placement in transposed text regions with a
// reference corner other than top left, the long form of referred-to
// segment counts, and the length of segments of unknown length.
//
// Corrupt data gives an error, never a panic; loops and allocations are
// bounded, where pdf.js might decode for minutes.
package jbig2
