# sonic-exporter logo assets

This package contains clean vector recreations of the selected sonic-exporter concept. The SVG files are the master assets; PNG files are convenience exports.

## Recommended files

- `svg/sonic-exporter-banner-dark.svg` — README hero/banner on a dark background.
- `svg/sonic-exporter-banner-light.svg` — README hero/banner on a light background.
- `svg/sonic-exporter-logo-dark.svg` — transparent horizontal logo for dark surfaces.
- `svg/sonic-exporter-logo-light.svg` — transparent horizontal logo for light surfaces.
- `svg/sonic-exporter-mark-dark.svg` — transparent icon for dark surfaces.
- `svg/sonic-exporter-mark-light.svg` — transparent icon for light surfaces.
- `png/sonic-exporter-icon-dark-512.png` — GitHub avatar, container artwork, or dashboard thumbnail.
- `png/sonic-exporter-social-preview-1280x640.png` — repository social preview.

## Suggested repository layout

```text
docs/
  assets/
    sonic-exporter-banner-dark.svg
    sonic-exporter-banner-light.svg
    sonic-exporter-logo-dark.svg
    sonic-exporter-logo-light.svg
    sonic-exporter-mark-dark.svg
    sonic-exporter-mark-light.svg
```

## README example

```html
<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="docs/assets/sonic-exporter-banner-dark.svg">
    <source media="(prefers-color-scheme: light)" srcset="docs/assets/sonic-exporter-banner-light.svg">
    <img src="docs/assets/sonic-exporter-banner-light.svg" alt="sonic-exporter — switch telemetry, ready to scrape" width="100%">
  </picture>
</p>
```

## Rules

- Prefer SVG wherever the destination supports it.
- Do not stretch, skew, recolour, or add extra effects.
- Keep clear space around the mark equal to roughly one port width.
- Use the icon without the tagline below about 500 px display width.
- Use the monochrome assets when gradients or multiple colours are unsuitable.

## Palette

- Cyan: `#00BCD4`
- Green: `#2EC6A3`
- Orange: `#FF8A1E`
- Navy: `#071622`
- Slate: `#687786`
- White: `#FFFFFF`

## Browser icon

- `svg/sonic-exporter-favicon.svg` is simplified for small sizes.
- `favicon.ico` contains 16, 32, and 48 px versions.

Example:

```html
<link rel="icon" href="docs/assets/sonic-exporter-favicon.svg" type="image/svg+xml">
<link rel="icon" href="docs/assets/favicon.ico" sizes="any">
```

## Source and final assets

The files in `source/` retain editable text where relevant. The files in `svg/` have the lettering converted to vector paths, so their appearance does not depend on an installed font. Use `svg/` for distribution.
