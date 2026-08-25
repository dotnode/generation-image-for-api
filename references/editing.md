# Editing and multi-image guidance

Read this guide for edits, reference-guided creation, compositing, localization, identity preservation, masks, or transparent-background requests.

## Establish roles before editing

Classify each input as one of:

- base image or edit target
- identity or subject reference
- object or wardrobe source
- style reference
- lighting or palette reference
- composition reference
- mask

The first `--edit` image is the base for a mask and should normally be the image whose framing must survive. Keep the prompt numbering identical to the CLI argument order.

## State changes and invariants separately

Every edit prompt should contain both:

```text
Changes: what the model is allowed to modify.
Invariants: identity, geometry, layout, text, objects, or styling that must remain unchanged.
```

Repeat the invariants in every correction request. Do not rely on the model remembering them implicitly.

## Identity preservation

For a person or character, explicitly preserve the relevant attributes:

- face and facial proportions
- apparent age and skin tone
- body shape and pose
- hair, expression, and gaze
- clothing or accessories not selected for replacement
- character proportions, palette, and signature details for illustrations

Use an identity reference as Image 1 when identity accuracy is more important than preserving another image's texture. Otherwise use the base composition as Image 1 and state which reference controls identity.

## Object replacement and removal

- Identify the exact object by location and appearance.
- Describe the replacement's material, scale, orientation, and interaction with nearby surfaces.
- Preserve surrounding texture, occlusion, reflections, and shadows.
- For removal, request plausible reconstruction of the concealed background.
- State that unrelated objects and framing must remain unchanged.

## Lighting, weather, and recoloring

Limit the requested change to environmental appearance:

- preserve identity, geometry, camera angle, and composition
- keep the same objects and spatial arrangement
- update light direction, shadow softness, reflections, atmosphere, precipitation, or palette consistently
- when recoloring a product or garment, preserve logos, seams, material texture, highlights, and folds unless otherwise requested

## Text localization or replacement

- Quote the source text and replacement text.
- Change only the specified text.
- Preserve layout, hierarchy, alignment, typography character, colors, and surrounding imagery.
- Allow line reflow only when required by the new language and ask for visually balanced spacing.
- Inspect every character after generation; image-model text can be imperfect.

## Compositing

For each inserted subject, specify:

- source image
- destination and approximate placement
- final scale and orientation
- foreground/background occlusion
- light direction and color temperature
- contact shadow and reflected light
- perspective, depth of field, grain, and edge sharpness

Preserve the base scene's crop and unrelated objects. Prevent extra copies of the inserted subject.

## Masks

- Use a mask only when the user supplies one or explicitly asks you to create one through an authorized image-processing step.
- Pass the mask with `--mask`; it applies to the first input image.
- Prefer a PNG mask with an alpha channel and matching dimensions.
- Treat the mask as guidance, not a guarantee of pixel-perfect boundaries.
- Repeat the allowed change and invariants in the prompt even when a mask is present.

## Transparent backgrounds and cutouts

The current Go CLI does not expose a dedicated background/transparency parameter. You may request transparency in the prompt, but do not promise that every third-party provider or model will return a real alpha channel.

After generation:

1. Inspect the result and file format.
2. Confirm whether the background is genuinely transparent rather than rendered as a checkerboard or solid color.
3. Check fine edges such as hair, fur, glass, fabric, and product labels.
4. If the provider cannot produce usable transparency, report that limitation instead of claiming success.

Do not silently switch models merely to obtain transparency. Explain the tradeoff when a different configured model is required.

## Iteration

Use small corrections:

- first correct identity or layout
- then lighting and integration
- then fine texture or text

Avoid asking for many unrelated corrections in one retry. A billable retry should have a specific reason and must preserve all previously satisfied invariants.
