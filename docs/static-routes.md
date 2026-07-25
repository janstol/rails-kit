# Static Routes

`rails-kit routes --static` parses route files directly in Go. It does not run Bundler, boot Rails, evaluate route conditions, or write the route cache. Use it when Rails cannot boot or when a fast approximate answer is more useful than exact runtime output.

```sh
rails-kit routes --static
rails-kit routes users --static
rails-kit routes --static --json
```

## Supported route DSL

The parser models:

- `resources`, `resource`, `namespace`, `scope`, `controller`, `member`, and `collection`
- `root`, standard verb routes, and `match` with static `via:` methods
- nesting, action filters, controller modules/defaults, helper prefixes, and custom member parameters
- recursive `draw` files and static block-defined route concerns
- literal redirects with optional numeric status codes
- constant and constant-qualified zero-argument mount points
- inline regular-expression constraints for named path parameters
- single-declaration inline blocks and comma-continued verb declarations

Generated resource helpers follow Rails inspector output: only the first enabled route sharing a helper displays its prefix. Multiple `match` methods use combined verbs such as `GET|POST`; `via: :all` and mount entries have an empty verb.

## Limitations

Static output is an approximation, not a replacement for `rails routes`. The parser does not fully model:

- routes drawn by gems
- internal engine or Rack application routes
- custom runtime `draw_paths` or `engine_name` values
- callable or parameterized concerns
- executable, dynamic, nested, or multi-statement inline blocks
- multiline or dynamic scopes, controller blocks, mounts, or `match` declarations
- receiver calls with arguments
- dynamic, callable, block, or option-hash redirects
- request, object, block, resource-level, and other unsupported constraints

Conditional mounts are retained with warnings because conditions are not evaluated. Unsupported or partially modeled DSL emits source-specific warnings on stderr while successfully parsed routes remain on stdout, including in JSON mode. Invalid, unreadable, unsafe, cyclic, or missing drawn files and concerns also produce warnings.

Use runtime `rails-kit routes` when exact route output is required for CI, production audits, or behavior that depends on executable application code.
