# Prompt construction

Use this guide when converting an image request into the prompt sent to the third-party image API.

## Preserve the user's intent

- If the request is already detailed, reorganize it without inventing new content.
- If it is brief, add only useful production details such as framing, lighting, intended use, or practical negative space.
- Do not invent extra people, products, slogans, brand colors, props, or story events.
- Ask a question only when a missing choice would materially change the result. Otherwise make a conservative assumption and proceed.

## Recommended order

Write complex prompts in this order:

```text
Purpose: where the image will be used
Primary request: the requested result
Input images: numbered roles, if any
Scene: setting or backdrop
Subject: focal subject and important attributes
Medium and style: photo, illustration, 3D, painted, and so on
Composition: viewpoint, crop, placement, and negative space
Lighting and mood: time, direction, softness, atmosphere
Palette and materials: only when relevant
Text: exact quoted wording, typography, and placement
Changes: what must change for an edit
Invariants: what must remain unchanged
Avoid: unwanted artifacts or additions
```

Use only the fields that help. A short clear prompt is better than a long prompt filled with irrelevant adjectives.

## Composition

- Specify close-up, medium, full-body, wide, overhead, low-angle, or eye-level when framing matters.
- Mention usable negative space when the image is intended for a banner, advertisement, landing page, or presentation.
- Do not choose left or right placement without a reason from the user's layout.
- For people, describe crop, gaze, pose, and interaction with important objects when those details matter.
- For products, request a clean silhouette, physically plausible materials, controlled reflections, and readable labels.

## Photorealism

When realism is requested, say so directly and use concrete physical details:

- natural skin and hair texture
- realistic fabric, wood, metal, glass, or plastic surfaces
- plausible lens perspective and depth of field
- coherent light direction, contact shadows, and reflections
- restrained processing unless the user requests a stylized commercial finish

## Text inside images

- Quote all required text exactly.
- Require verbatim rendering with no added or omitted characters.
- State font character, size hierarchy, color, alignment, and placement when relevant.
- For uncommon words, include a letter-by-letter spelling.
- Prefer `medium` or higher quality for dense labels, infographics, packaging copy, and interface mockups.

## Reference images

Number each image and name its role. Describe what information may be borrowed from it and what must not be copied or changed.

Example:

```text
Image 1: identity reference; preserve facial structure, hair, and age.
Image 2: wardrobe reference only; use the coat design and material.
Image 3: lighting reference only; use the warm rim light, not its composition.
```

## Common prompt patterns

### New image

```text
Purpose: landing-page hero image
Primary request: a premium studio photograph of a matte ceramic coffee mug
Composition: wide crop with uncluttered negative space for page copy
Lighting: soft directional studio light with controlled shadows
Materials: realistic matte ceramic and subtle tabletop texture
Avoid: logos, text, watermark, excessive reflections
```

### Precise edit

```text
Image 1: edit target
Changes: replace only the background with a rainy neon street at night
Invariants: preserve the person, face, pose, hair, clothing, edges, crop, and camera perspective
Integration: match reflected neon light and contact shadows without altering identity
Avoid: extra people, text, watermark, facial retouching
```

### Multi-image composite

```text
Image 1: base scene and final framing
Image 2: subject to insert
Changes: place the subject from Image 2 into Image 1 near the existing table
Integration: match scale, perspective, lens blur, color temperature, light direction, and contact shadows
Invariants: keep Image 1 composition and unrelated objects unchanged; keep Image 2 identity and clothing unchanged
Avoid: duplicated limbs, halos, added objects, text, watermark
```

## Validation after generation

Inspect the result against the prompt:

- correct subject and number of subjects
- requested framing and aspect ratio
- coherent lighting, shadows, reflections, and perspective
- exact required text and no extra text
- preserved identity and edit invariants
- clean edges for compositing or cutouts
- no unexpected logos, watermarks, duplicated anatomy, or unrelated objects

When a correction is needed, change one aspect at a time and repeat the critical invariants.
