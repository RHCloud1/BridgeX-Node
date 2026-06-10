# sing-box Compatibility Strategy

sing-box changes quickly, so NodeBridge should not treat its JSON config as a timeless format.

Official sing-box documentation marks several old fields as deprecated or removed across recent versions. Examples include GeoIP/Geosite removal in `1.12.0`, legacy inbound sniff fields moving toward rule actions, and DNS/routing migrations. This means a config that works on one release line can fail after an automatic core update.

## Project Rule

NodeBridge should render native sing-box configuration through versioned renderers:

| Renderer | Target | Purpose |
| --- | --- | --- |
| `sing-box-1.12` | stable 1.12 line | Default production target until newer schemas are proven. |
| `sing-box-1.13` | stable 1.13 line | Adds newer DNS/routing behavior after validation. |
| `sing-box-next` | latest/alpha testing | Development only. |

The config file chooses the renderer explicitly:

```json
{
  "type": "sing-box",
  "version_policy": "pinned",
  "target_version": "1.12",
  "renderer": "sing-box-1.12"
}
```

## Render And Validate Flow

1. Normalize panel or subscription data into `domain.Node`.
2. Select renderer by `kernels[].renderer`.
3. Write the candidate native config to a temporary path.
4. Run `sing-box check -c candidate.json`.
5. Replace the active config only after validation succeeds.
6. Restart the kernel.

If validation fails, keep the old config and report the error through `/v1/kernels`.

## Avoid These Patterns

- Do not hardcode deprecated `geoip` or `geosite` rules into generated configs.
- Do not auto-update sing-box across major/minor lines without config validation.
- Do not mix renderer assumptions with panel parsing code.
- Do not restart a live kernel until the replacement config has passed `check`.

## Useful Official References

- sing-box deprecated options: https://sing-box.sagernet.org/deprecated/
- sing-box migration guide: https://sing-box.sagernet.org/migration/
- sing-box changelog: https://sing-box.sagernet.org/changelog/

