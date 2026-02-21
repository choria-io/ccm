+++
title = "Design Documents"
description = "LLM generated design documents for resources"
toc = true
weight = 80
pre = "<b>8. </b>"
+++

Design documents provide detailed implementation guidance for CCM's resource types, providers, and internal components. They are intended for developers contributing to CCM or those seeking to understand specific implementation details.

For end-user documentation on how to use resources, see [Resources](../resources/).

> [!info] Note
> These design documents are largely written with AI assistance and reviewed before publication.

## Contents

Each design document covers:

- **Purpose and scope**: What the component does and its responsibilities
- **Architecture**: How the component fits into CCM's overall design
- **Implementation details**: Key data structures, interfaces, and algorithms
- **Provider contracts**: Requirements for implementing new providers
- **Testing considerations**: How to test the component

## Available Documents

| Document            | Description                                              |
|---------------------|----------------------------------------------------------|
| [Archive](archive/) | Archive resource for downloading and extracting archives |
| [Exec](exec/)       | Exec resource for command execution                      |
| [File](file/)       | File resource for managing files and directories         |
| [Package](package/) | Package resource for system package management           |
| [Service](service/) | Service resource for system service management           |
 | [New Type](new/)    | How to add a new resource type to CCM                    |

