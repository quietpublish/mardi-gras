# Downgrade an OpenAPI 3.1 spec to 3.0.x so oapi-codegen (which does not yet
# fully support 3.1, see oapi-codegen#373) can generate a Go client.
#
# Handles the 3.1-isms gascity's spec uses:
#   - `type: [T, "null"]`  -> `type: T` + `nullable: true`
#   - `exclusiveMinimum`/`exclusiveMaximum` as numbers -> boolean + min/max
#   - strips `$schema`, `const`, `prefixItems`, `unevaluatedProperties`
# Run as the first step of `make gc-client`; the committed openapi.json stays
# the authentic pinned 3.1 contract.
walk(
  if type == "object" then
    ( if (.type | type) == "array"
      then
        ( [ .type[] | select(. != "null") ] ) as $nonnull
        | ( .type | any(. == "null") ) as $hasnull
        | .type = ( $nonnull[0] // "object" )
        | ( if $hasnull then . + {nullable: true} else . end )
      else . end )
    | ( if (.exclusiveMinimum | type) == "number"
        then .minimum = .exclusiveMinimum | .exclusiveMinimum = true
        else . end )
    | ( if (.exclusiveMaximum | type) == "number"
        then .maximum = .exclusiveMaximum | .exclusiveMaximum = true
        else . end )
    | del(.["$schema"], .const, .prefixItems, .unevaluatedProperties)
  else . end
)
| .openapi = "3.0.3"
