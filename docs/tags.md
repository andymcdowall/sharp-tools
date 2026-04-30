# Tag Normalization Hints

This document provides guidance for tag normalization in the intent parser.

## Tag Normalization Rules

1. **Lowercase**: All tags should be lowercase
2. **Hyphenated**: Use hyphens (-) instead of underscores or spaces
3. **No Duplicates**: Remove duplicate tags
4. **Preferred Tags**: When possible, use tags from the preferred tag list

## Primary Operation Tags

Primary operation tags represent the main action. Common operations include:

### Image Operations
- convert, resize, crop, rotate, flip, mirror
- compress, optimize, strip-metadata, watermark
- thumbnail, upscale, downscale, grayscale
- blur, sharpen, denoise, colorize

### Video Operations
- transcode, trim-video, split-video, merge-video
- extract-frames, extract-audio, add-subtitles
- mux, demux, remux, stabilize

### Audio Operations
- transcode-audio, trim-audio, split-audio
- normalize, amplify, noise-reduce
- pitch-shift, tempo-change

### Text Operations
- strip, replace, extract, truncate
- wrap, unwrap, indent, dedent
- lowercase, uppercase, title-case
- camel-case, kebab-case, snake-case

### Data Operations
- parse, serialize, validate, transform
- filter, map, reduce, group-by

### Format Conversion
- json-to-csv, csv-to-json
- json-to-yaml, yaml-to-json
- pdf-to-text, markdown-to-html

### Compression & Archiving
- decompress, archive, extract-archive

### Security & Crypto
- encrypt, decrypt, hash-file
- base64-encode, base64-decode

## Input/Output Types

- **file**: File path or file reference
- **text**: Plain text content
- **data**: Structured data (JSON, YAML, etc.)
- **none**: No input/output

## Examples

| Raw Input | Normalized Tags |
|----------|-----------------|
| "Convert PNG to JPG" | convert, png, jpg |
| "Resize image to 100x100" | resize, image |
| "Compress PDF file" | compress, pdf |
| "Generate QR code" | qr-generate, image |
| "Parse JSON data" | parse, json, data |