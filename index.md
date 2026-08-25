---
layout: default
title: Linode COSI Driver
nav_order: 1
---

{%- capture readme -%}
{%- include_relative README.md -%}
{%- endcapture -%}

{{ readme
  | replace: ".md)", ".html)"
  | replace: ".md#", ".html#"
}}
