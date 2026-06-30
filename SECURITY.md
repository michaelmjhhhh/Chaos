# Security Policy

## Supported versions

Chaos is distributed as a rolling release. Only the latest tagged version
receives security fixes. Please make sure you are on the most recent
[release](https://github.com/michaelmjhhhh/Chaos/releases/latest) before
reporting an issue.

## Reporting a vulnerability

Please report security issues **privately** rather than opening a public issue:

- Use GitHub's [private vulnerability reporting](https://github.com/michaelmjhhhh/Chaos/security/advisories/new), or
- Email the maintainer at the address on the commit history.

Include enough detail to reproduce (affected component, steps, and impact). You
can expect an acknowledgement within a few days. Please give a reasonable window
to release a fix before any public disclosure.

## Scope notes

- **Hosted free-trial proxy** (`hosted/`): the app token bundled in the client is
  intentionally public and is only a light gate. Real protection is server-side
  per-device and global usage limits plus per-call cost ceilings. Trial-abuse
  reports are welcome, but the bundled token being readable is by design.
- **Local app**: Chaos reads images from a watched folder and writes renamed
  copies to your chosen output folder. It sends image data only to the vision
  provider you configure.
