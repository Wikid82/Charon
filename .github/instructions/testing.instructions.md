---
applyTo: '**'
description: 'Testing instructions for code and content validation.'
---
# Testing Instructions

1. Never run tests using tail or head flags that limit the output. Some tests require user interaction and hang if output is truncated. Always run tests in a way that allows full output to be visible.

2. When running tests, don't holucinate paths or filenames. Always use prebuilt tasks. If there is not a task for repetitive testing, create one that fits the project's conventions.

3. When tests fail, always analyze the output carefully. Look for stack traces, error messages, and any hints that indicate what went wrong. Avoid making assumptions without evidence from the test results. Don't fix the test to run based on the code as it may be failing because there is a bug in the code. Always research to make sure if its the code that needs fixed or the test.

4. It is fine to run just the tests when creating and troubleshooting but the last step before completion is to run the tests with coverage to make sure we stay above the coverage threshold.
