+++
title = "Docs Style Guide"
description = "Define writing conventions for CCM documentation"
toc = true
weight = 110
+++

This guide describes the writing conventions used throughout the CCM documentation. Follow these rules when adding or editing pages.

All sections apply to every documentation page. The Page structure section applies only to resource reference pages under `resources/`.

## Voice and tone

- Write in plain, direct North American English.
- Use the present tense and active voice: "The service resource manages system services," not "System services are managed by the service resource."
- Address the reader implicitly. Do not use "you" or "we". State facts and give instructions: "Specify commands with their full path," not "You should specify commands with their full path."
- Keep sentences short. One idea per sentence.
- Do not editorialize or use filler ("Note that," "It is important to," "Simply").
- Do not use emojis.
- Do not use em dashes. Use commas, periods, or semicolons instead.

## Page structure

Every resource page follows this order:

1. **Front matter**: TOML (`+++`) with `title`, `description`, `toc = true`, and `weight`.
2. **Opening paragraph**: One or two sentences stating what the resource does.
3. **Callout**: A warning or note about common pitfalls, using `> [!info]` syntax.
4. **Primary example**: A tabbed block (Manifest / CLI / API Request) showing typical usage.
5. **Brief explanation**: One or two sentences describing what the example does.
6. **Ensure values**: Table of valid `ensure` states.
7. **Properties**: Table of all properties with short descriptions.
8. **Additional sections**: Provider notes, idempotency, authentication, behavioral details as needed.

## Front matter

Use TOML delimiters (`+++`). Include at minimum:

```toml
+++
title = "Resource Name"
description = "Short verb-phrase summary"
toc = true
weight = 30
+++
```

The `description` field should read as a phrase, not a sentence. No trailing period. Start with a verb: "Manage files content, ownership and more."

## Headings

- Use `##` for top-level sections within a page. Do not use `#` (the page title comes from front matter).
- Use `###` for subsections.
- Use sentence case: "Guard commands," not "Guard Commands."
- Keep headings short and descriptive.

## Tables

Use Markdown tables for structured reference content: ensure values, properties, provider lists.

- The first column is the property or value name in backticks.
- The second column is a brief description, written as a sentence fragment with no trailing period.
- Align columns with pipes for readability.

```markdown
| Property | Description                          |
|----------|--------------------------------------|
| `name`   | Absolute path to the file            |
| `ensure` | Desired state (`present`, `absent`)  |
```

## Code examples

### Tabbed blocks

Show every example in three tabs using Hugo shortcodes:

1. **Manifest**: YAML manifest syntax.
2. **CLI**: `ccm ensure` command using `nohighlight` fence.
3. **API Request**: JSON request body.

```
{{</* tabs */>}}
{{%/* tab title="Manifest" */%}}
...
{{%/* /tab */%}}
{{%/* tab title="CLI" */%}}
...
{{%/* /tab */%}}
{{%/* tab title="API Request" */%}}
...
{{%/* /tab */%}}
{{</* /tabs */>}}
```

Not every example needs all three tabs. Secondary examples deeper in a page may show only the most relevant format.

### YAML

- Use realistic but minimal values.
- Quote version strings and octal modes: `"5.9"`, `"0644"`.

### CLI

- Use `nohighlight` as the fence language.
- Use backslash continuations for long commands.
- Add a brief comment above the command when context is needed.

### JSON

- Use `json` as the fence language.
- Always include the `protocol` and `type` fields in API examples.

## Callouts

Use the `> [!info]` blockquote syntax for warnings and notes:

```markdown
> [!info] Warning
> Use absolute file paths and primary group names.
```

```markdown
> [!info] Note
> The provider will not run `apt update` before installing a package.
```

Use **Warning** for constraints the reader must follow to avoid errors. Use **Note** for supplementary information. A custom label may replace `Warning` or `Note` when it adds clarity, such as `> [!info] Default Hierarchy`.

## Version badges

Mark features with the CCM release that introduced them using a Hugo `badge` shortcode. Place the badge immediately after the section heading or, for new properties, at the end of the description cell:

```markdown
## Manage attributes only {{% badge style="primary" title="Version" %}}0.0.29{{% /badge %}}

| `force` (boolean) | Allow `ensure: absent` to remove non-empty directories {{% badge style="primary" title="Version" %}}0.0.28{{% /badge %}} |
```

Use `style="primary"` and `title="Version"`. The badge body is the release tag without a leading `v`.

Add a version badge only when a feature is introduced. Do not retroactively badge pre-existing content, and remove the badge once the release in question is several versions behind current.

## Descriptions and explanations

- After a tabbed example block, add one or two sentences explaining what the example does and why.
- Describe behavior, not implementation: "The command runs only if `/tmp/hello` does not exist," not "The code checks whether the file exists and skips execution if found."
- When describing how multiple options interact, use a truth table.

## Terminology

- Use "resource," "provider," "property," "manifest" consistently.
- Refer to ensure states and property names in backticks: `present`, `name`, `ensure`.
- Reference other resources using the `type#name` notation in backticks: `package#httpd`.
- When cross-referencing other documentation pages, use relative Hugo links.

## General formatting

- No trailing whitespace.
- One blank line between sections.
- No blank line between a heading and its first paragraph.
- Wrap inline code, file paths, command names, property names, and values in backticks.
- Do not use bold or italic for emphasis in reference content. Reserve bold for definition list terms within prose.
