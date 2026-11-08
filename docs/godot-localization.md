# Public Godot translations

`GamePOC.godot_targets` is a list of server-project-relative Godot project directories.
`ang game-poc` emits `generated/translations/<locale>.po` in each target, then registers
them in that project's `project.godot`. ANG owns the three `locale/translations`,
`locale/fallback`, and `locale/test` settings. Default and fallback come from the
world localization catalog. PO msgid is the key; msgstr is text; extracted comments
carry the public key's translator context. Only `ui.*` is exported. Story and map
knowledge must travel through the authorized server views, never these files.
Missing translations resolve through catalog fallback then default at compilation.

CUE `{name}` and `{0}` placeholders map to Godot `String.format` with a dictionary
or an array. PO preserves all braces, including escaped `{{`/`}}`; callers needing
literal braces must unescape those after formatting. CUE's numeric format suffixes
(e.g. `{0:0.0}`) require explicit presentation formatting before substitution;
Godot String.format does not implement .NET numeric format specifications.

`world.world_text.metrics_dir` is relative to the server project and points at
engine-neutral output of the font tooling. The legacy `metrics: "WorldText/style"`
identifies the resource style; ANG reads `<metrics_dir>/<style>.json`, with no
Unity Resources dependency. `generated/presentation.json` is the public presentation
resource for Godot calibration, emitted directly alongside translations.

## Theme resources

`world.godot_theme` is optional and is decoded independently of world runtime
serialization. `target` is server-relative, `resource_name` names the default
Theme file, and `default_skin` selects one of `skins`. Each skin supplies named
sRGB roles. Font assets declare `id`, `role`, `family`, local `res://` file,
SHA-256, `license`, licence file/SHA and Cyrillic support. Texture assets declare
file/SHA. Assets are read and checked, never synthesized.

`tokens` contains named numeric sizes/spacings. Ordered `resources` define typed
StyleBoxFlat/Texture/Empty subresources; ordered `properties` define the Theme
and its type variations. A property has exactly one of `color` (skin role),
`token`, `external`, `internal`, or `string_name`. Optional `alpha` and `float`
control colour opacity and numeric serialization. Unknown references, changed
assets, duplicate IDs/properties and ambiguous values fail generation.

`min_text_px` must be at least 12. Every declared font-size property is checked
against it. `contrast` lists named foreground/background role pairs; ANG emits
`generated/theme-contrast.json` with WCAG relative luminance ratios and AA 4.5
flags for each skin. Contrast is reported without modifying approved colours.
Ratios describe declared opaque colour swatches; textured or translucent
backgrounds require separate image-level accessibility review.

The game CUE schema is `content/world/godot_theme_schema.cue`; values are in
`godot_theme.cue`. Caste records own `roles.bark_outline` and `roles.ui`, with the
old `colour` derived from `roles.bark_outline`. UI and bark hues are independent.

World-lettering caveat: native Godot TTF shaping is not equivalent to the existing
atlas-tool metrics. In the game, a native 496-line probe found up to 7.838% width
difference, and PT Sans Narrow does not contain the atlas tool's drawn arrows.
Those metrics cannot certify Label3D fit. See the game's Phase 3 report before
implementing a world text renderer.
