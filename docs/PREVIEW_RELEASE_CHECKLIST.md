# ARTEX English edition source prerelease verification

The `v0.1.0-alpha.1` release contains source only. It does not claim production readiness or reliable live-target behavior.

- The source archive contains the root AGPL-3.0 license, frontend MIT notice, upstream attribution, and the HumanWill English-edition provenance.
- The active HumanWill responsible-use guidance is consistent across English, Korean, and Chinese. Earlier source-edition notices are labeled as historical context.
- The release notes disclose unresolved positive finding/report and unavailable-service workflow failures, 24 frontend dependency advisories including two critical, and platform checks that remain incomplete.
- The local validation record includes a clean-source build on macOS Intel, a later frontend static build after the guidance revision, focused self-updater and localization checks, and document-link checks. These do not establish safety or performance on real targets.
- GitHub Actions are disabled. The checked-in tag-triggered workflow and inherited Docker destination require separate review before enabling automation or publishing binaries/images.
- There are no prebuilt application binaries or container images in this prerelease. The self-updater queries stable releases only and does not install this source-only prerelease.
