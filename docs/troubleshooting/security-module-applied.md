---
title: Checking that a security module is applied
description: What the "not applied" line on the Security page means for the WAF and rate limit, and how to fix it.
---

## Checking that a security module is applied

The **Security** page tells you whether the WAF and the Rate Limiting switch are working on every site they should cover. When a module is on but cannot cover every site, a short line under its card says so and gives a reason.

No line means the module is on and working on every site, or the module is off.

### What the reasons mean

| Line on the card | What it means | How to fix it |
| --- | --- | --- |
| No rule set exists yet | The WAF is on, but there is no rule set to load. | Open the **WAF** page and add a rule set. |
| No rule set is chosen | You have rule sets, but none is picked for the WAF. | Open the **WAF** page and pick a rule set from the list, or choose automatic selection. |
| The chosen rule set cannot be loaded | The rule set the WAF uses has no rule file ready. | Open the **WAF** page, check the rule set, or choose a different one. |
| No request count or time window is set | The rate limit is on, but its numbers are empty or zero. | Open the Rate Limiting card, enter the numbers, and save. |

### Steps

1. Open the **Security** page.
2. Find the card for the module (**Web Application Firewall** or **Rate Limiting**).
3. Read the line under the card, if there is one.
4. Use the fix from the table above.
5. Go back to the **Security** page and check that the line is gone.

### Notes

- A rule set can be chosen by its name, or by an address that starts with `https://`.
- Turning a module on by itself fills in any missing rate limit numbers with the standard values: 100 requests, a 60 second window and a burst of 20.
- Rule set modes (Detection or Blocking) are set on the **WAF** page. Detection logs matching requests. Blocking stops them.

### Still stuck?

If the line does not change after you follow the steps, save the settings once more, then check the Security page again. If it still shows the same line, note the exact wording and ask for help on the Charon project page.
