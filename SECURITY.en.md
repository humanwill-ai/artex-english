# Security Policy

English policy for the HumanWill edition · [Korean source-edition policy](SECURITY.md)

This repository is currently private and has no published HumanWill release. See [the current README](README.md) for project status.

This document explains how to report security vulnerabilities in the **HumanWill ARTEX English edition (`humanwill-ai/artex-en`) code itself**. ARTEX is an offensive-security tool that performs penetration testing, but what this document covers is not the results of attacking something with the tool; it is **vulnerabilities that arise when you operate or deploy this software**.

For example:

- Authentication bypass, privilege escalation, SSRF, or injection in the web UI / API server (`server/`)
- Exposure or plaintext storage of stored credentials or API keys
- Flaws that let the agent skip the human-in-the-loop approval step and run tools outside its authorized scope
- Supply-chain or dependency vulnerabilities

## How to report

**Do not file security vulnerabilities as public issues.** Once public, they can be exploited before a patch is available. Please use one of the following private channels instead.

1. If available in **humanwill-ai/artex-en**, use **Security** → **"Report a vulnerability"** (GitHub Private Vulnerability Reporting). Availability is not assumed during private preparation.
2. If that feature is not enabled, do not write sensitive details. Open only a minimal issue stating that you would like a private security contact, and ask the maintainers to open a private channel.

Including the following in your report speeds up triage:

- The affected component and version (release tag or commit hash)
- Reproduction steps and impact (what it lets an attacker do)
- If possible, a proof of concept (PoC) and a suggested mitigation

## Handling process

- Once we receive a report, we reply to acknowledge it within a reasonable time and assess its validity and severity.
- When a fix is ready, we coordinate with the reporter on the public disclosure timing as a **coordinated disclosure**. We do not disclose details before a fix is available.
- With your consent, we credit your contribution when we disclose.

## Supported scope

This repository is the **HumanWill English localization of jiwoochris/artex-ko, derived from Autumn-27/ARTEX**. Security fixes are provided against the **latest default branch**. We do not guarantee backports to earlier releases. If a vulnerability also affects upstream code, coordinate with the HumanWill maintainers on any report to [jiwoochris/artex-ko](https://github.com/jiwoochris/artex-ko) or [Autumn-27/ARTEX](https://github.com/Autumn-27/ARTEX). Do not assume an upstream reporting channel is available.

## Out of scope

The following are **not** covered by this security policy.

- A vulnerability in **an external system** that you discovered by attacking it with ARTEX. That is something to report to the owner of that system.
- Problems caused by running the tool against someone else's system without authorization. Such use itself violates the [usage scope](README.en.md#️-read-first--authorized-use-and-legal-notice) and domestic law.
- The mere fact that ARTEX runs an offensive tool "by design." This tool is built to perform penetration testing within an authorized scope.

If you witness **misuse** of this tool (use beyond the authorized scope), do not report it through GitHub. Use the lawful reporting channels appropriate to the conduct (the owner of the affected system or the relevant authorities).
