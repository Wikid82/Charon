#!/usr/bin/env python3
"""
Agent Skills Frontmatter Validator

Validates YAML frontmatter in .SKILL.md files against the agentskills.io
specification. Ensures required fields are present, formats are correct,
and custom metadata follows project conventions.

Usage:
    python3 validate-skills.py [path/to/.github/skills/]
    python3 validate-skills.py --single path/to/skill.SKILL.md

Exit Codes:
    0 - All validations passed
    1 - Validation errors found
    2 - Script error (missing dependencies, invalid arguments)
"""

import os
import sys
import re
import argparse
from pathlib import Path
from typing import List, Dict, Tuple, Any, Optional

try:
    import yaml
except ImportError:
    print("Error: PyYAML is required. Install with: pip install pyyaml", file=sys.stderr)
    sys.exit(2)


# Validation rules
REQUIRED_FIELDS = ["name", "version", "description", "author", "license", "tags"]
VALID_CATEGORIES = ["test", "integration-test", "security", "qa", "build", "utility", "docker"]
VALID_EXECUTION_TIMES = ["short", "medium", "long"]
VALID_RISK_LEVELS = ["low", "medium", "high"]
VALID_OS_VALUES = ["linux", "darwin", "windows"]
VALID_SHELL_VALUES = ["bash", "sh", "zsh", "powershell", "cmd"]

VERSION_REGEX = re.compile(r'^\d+\.\d+\.\d+$')
NAME_REGEX = re.compile(r'^[a-z][a-z0-9-]*$')


class ValidationError:
    """Represents a validation error with context."""

    def __init__(self, skill_file: str, field: str, message: str, severity: str = "error"):
        self.skill_file = skill_file
        self.field = field
        self.message = message
        self.severity = severity

    def __str__(self) -> str:
        return f"[{self.severity.upper()}] {self.skill_file} :: {self.field}: {self.message}"


class SkillValidator:
    """Validates Agent Skills frontmatter."""

    def __init__(self, strict: bool = False):
        self.strict = strict
        self.errors: List[ValidationError] = []
        self.warnings: List[ValidationError] = []

    def validate_file(self, skill_path: Path) -> Tuple[bool, List[ValidationError]]:
        """Validate a single SKILL.md file."""
        try:
            with open(skill_path, 'r', encoding='utf-8') as f:
                content = f.read()
        except Exception as e:
            return False, [ValidationError(str(skill_path), "file", f"Cannot read file: {e}")]

        # Extract frontmatter
        frontmatter = self._extract_frontmatter(content)
        if not frontmatter:
            return False, [ValidationError(str(skill_path), "frontmatter", "No valid YAML frontmatter found")]

        # Parse YAML
        try:
            data = yaml.safe_load(frontmatter)
        except yaml.YAMLError as e:
            return False, [ValidationError(str(skill_path), "yaml", f"Invalid YAML: {e}")]

        if not isinstance(data, dict):
            return False, [ValidationError(str(skill_path), "yaml", "Frontmatter must be a YAML object")]

        # Run validation checks
        file_errors: List[ValidationError] = []
        file_errors.extend(self._validate_required_fields(skill_path, data))
        file_errors.extend(self._validate_name(skill_path, data))
        file_errors.extend(self._validate_version(skill_path, data))
        file_errors.extend(self._validate_description(skill_path, data))
        file_errors.extend(self._validate_tags(skill_path, data))
        file_errors.extend(self._validate_compatibility(skill_path, data))
        file_errors.extend(self._validate_metadata(skill_path, data))

        # Separate errors and warnings
        errors = [e for e in file_errors if e.severity == "error"]
        warnings = [e for e in file_errors if e.severity == "warning"]

        self.errors.extend(errors)
        self.warnings.extend(warnings)

        return len(errors) == 0, file_errors

    def _extract_frontmatter(self, content: str) -> Optional[str]:
        """Extract YAML frontmatter from markdown content."""
        if not content.startswith('---\n'):
            return None

        end_marker = content.find('\n---\n', 4)
        if end_marker == -1:
            return None

        return content[4:end_marker]

    def _validate_required_fields(self, skill_path: Path, data: Dict) -> List[ValidationError]:
        """Check that all required fields are present."""
        errors = []
        for field in REQUIRED_FIELDS:
            if field not in data:
                errors.append(ValidationError(
                    str(skill_path), field, f"Required field missing"
                ))
            elif not data[field]:
                errors.append(ValidationError(
                    str(skill_path), field, f"Required field is empty"
                ))
        return errors

    def _validate_name(self, skill_path: Path, data: Dict) -> List[ValidationError]:
        """Validate name field format."""
        errors = []
        if "name" in data:
            name = data["name"]
            if not isinstance(name, str):
                errors.append(ValidationError(
                    str(skill_path), "name", "Must be a string"
                ))
            elif not NAME_REGEX.match(name):
                errors.append(ValidationError(
                    str(skill_path), "name",
                    "Must be kebab-case (lowercase, hyphens only, start with letter)"
                ))

            # Check filename matches name
            expected_filename = f"{name}.SKILL.md"
            if skill_path.name != expected_filename:
                errors.append(ValidationError(
                    str(skill_path), "name",
                    f"Filename should be '{expected_filename}' to match name field",
                    severity="warning"
                ))
        return errors

    def _validate_version(self, skill_path: Path, data: Dict) -> List[ValidationError]:
        """Validate version field format."""
        errors = []
        if "version" in data:
            version = data["version"]
            if not isinstance(version, str):
                errors.append(ValidationError(
                    str(skill_path), "version", "Must be a string"
                ))
            elif not VERSION_REGEX.match(version):
                errors.append(ValidationError(
                    str(skill_path), "version",
                    "Must follow semantic versioning (x.y.z)"
                ))
        return errors

    def _validate_description(self, skill_path: Path, data: Dict) -> List[ValidationError]:
        """Validate description field."""
        errors = []
        if "description" in data:
            desc = data["description"]
            if not isinstance(desc, str):
                errors.append(ValidationError(
                    str(skill_path), "description", "Must be a string"
                ))
            elif len(desc) > 120:
                errors.append(ValidationError(
                    str(skill_path), "description",
                    f"Must be 120 characters or less (current: {len(desc)})"
                ))
            elif '\n' in desc:
                errors.append(ValidationError(
                    str(skill_path), "description", "Must be a single line"
                ))
        return errors

    def _validate_tags(self, skill_path: Path, data: Dict) -> List[ValidationError]:
        """Validate tags field."""
        errors = []
        if "tags" in data:
            tags = data["tags"]
            if not isinstance(tags, list):
                errors.append(ValidationError(
                    str(skill_path), "tags", "Must be a list"
                ))
            elif len(tags) < 2:
                errors.append(ValidationError(
                    str(skill_path), "tags", "Must have at least 2 tags"
                ))
            elif len(tags) > 5:
                errors.append(ValidationError(
                    str(skill_path), "tags",
                    f"Must have at most 5 tags (current: {len(tags)})",
                    severity="warning"
                ))
            else:
                for tag in tags:
                    if not isinstance(tag, str):
                        errors.append(ValidationError(
                            str(skill_path), "tags", "All tags must be strings"
                        ))
                    elif tag != tag.lower():
                        errors.append(ValidationError(
                            str(skill_path), "tags",
                            f"Tag '{tag}' should be lowercase",
                            severity="warning"
                        ))
        return errors

    def _validate_compatibility(self, skill_path: Path, data: Dict) -> List[ValidationError]:
        """Validate compatibility section."""
        errors = []
        if "compatibility" in data:
            compat = data["compatibility"]
            if not isinstance(compat, dict):
                errors.append(ValidationError(
                    str(skill_path), "compatibility", "Must be an object"
                ))
            else:
                # Validate OS
                if "os" in compat:
                    os_list = compat["os"]
                    if not isinstance(os_list, list):
                        errors.append(ValidationError(
                            str(skill_path), "compatibility.os", "Must be a list"
                        ))
                    else:
                        for os_val in os_list:
                            if os_val not in VALID_OS_VALUES:
                                errors.append(ValidationError(
                                    str(skill_path), "compatibility.os",
                                    f"Invalid OS '{os_val}'. Valid: {VALID_OS_VALUES}",
                                    severity="warning"
                                ))

                # Validate shells
                if "shells" in compat:
                    shells = compat["shells"]
                    if not isinstance(shells, list):
                        errors.append(ValidationError(
                            str(skill_path), "compatibility.shells", "Must be a list"
                        ))
                    else:
                        for shell in shells:
                            if shell not in VALID_SHELL_VALUES:
                                errors.append(ValidationError(
                                    str(skill_path), "compatibility.shells",
                                    f"Invalid shell '{shell}'. Valid: {VALID_SHELL_VALUES}",
                                    severity="warning"
                                ))
        return errors

    def _validate_metadata(self, skill_path: Path, data: Dict) -> List[ValidationError]:
        """Validate custom metadata section."""
        errors = []
        if "metadata" not in data:
            return errors  # Metadata is optional

        metadata = data["metadata"]
        if not isinstance(metadata, dict):
            errors.append(ValidationError(
                str(skill_path), "metadata", "Must be an object"
            ))
            return errors

        # Validate category
        if "category" in metadata:
            category = metadata["category"]
            if category not in VALID_CATEGORIES:
                errors.append(ValidationError(
                    str(skill_path), "metadata.category",
                    f"Invalid category '{category}'. Valid: {VALID_CATEGORIES}",
                    severity="warning"
                ))

        # Validate execution_time
        if "execution_time" in metadata:
            exec_time = metadata["execution_time"]
            if exec_time not in VALID_EXECUTION_TIMES:
                errors.append(ValidationError(
                    str(skill_path), "metadata.execution_time",
                    f"Invalid execution_time '{exec_time}'. Valid: {VALID_EXECUTION_TIMES}",
                    severity="warning"
                ))

        # Validate risk_level
        if "risk_level" in metadata:
            risk = metadata["risk_level"]
            if risk not in VALID_RISK_LEVELS:
                errors.append(ValidationError(
                    str(skill_path), "metadata.risk_level",
                    f"Invalid risk_level '{risk}'. Valid: {VALID_RISK_LEVELS}",
                    severity="warning"
                ))

        # Validate boolean fields
        for bool_field in ["ci_cd_safe", "requires_network", "idempotent"]:
            if bool_field in metadata:
                if not isinstance(metadata[bool_field], bool):
                    errors.append(ValidationError(
                        str(skill_path), f"metadata.{bool_field}",
                        "Must be a boolean (true/false)",
                        severity="warning"
                    ))

        return errors

    def validate_directory(self, skills_dir: Path) -> bool:
        """Validate all SKILL.md files in a directory."""
        if not skills_dir.exists():
            print(f"Error: Directory not found: {skills_dir}", file=sys.stderr)
            return False

        skill_files = list(skills_dir.glob("*.SKILL.md"))
        if not skill_files:
            print(f"Warning: No .SKILL.md files found in {skills_dir}", file=sys.stderr)
            return True  # Not an error, just nothing to validate

        print(f"Validating {len(skill_files)} skill(s)...\n")

        success_count = 0
        for skill_file in sorted(skill_files):
            is_valid, _ = self.validate_file(skill_file)
            if is_valid:
                success_count += 1
                print(f"✓ {skill_file.name}")
            else:
                print(f"✗ {skill_file.name}")

        # Print summary
        print(f"\n{'='*70}")
        print(f"Validation Summary:")
        print(f"  Total skills: {len(skill_files)}")
        print(f"  Passed: {success_count}")
        print(f"  Failed: {len(skill_files) - success_count}")
        print(f"  Errors: {len(self.errors)}")
        print(f"  Warnings: {len(self.warnings)}")
        print(f"{'='*70}\n")

        # Print errors
        if self.errors:
            print("ERRORS:")
            for error in self.errors:
                print(f"  {error}")
            print()

        # Print warnings
        if self.warnings:
            print("WARNINGS:")
            for warning in self.warnings:
                print(f"  {warning}")
            print()

        return len(self.errors) == 0


def main():
    parser = argparse.ArgumentParser(
        description="Validate Agent Skills frontmatter",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog=__doc__
    )
    parser.add_argument(
        "path",
        nargs="?",
        default=".github/skills",
        help="Path to .github/skills directory or single .SKILL.md file (default: .github/skills)"
    )
    parser.add_argument(
        "--strict",
        action="store_true",
        help="Treat warnings as errors"
    )
    parser.add_argument(
        "--single",
        action="store_true",
        help="Validate a single .SKILL.md file instead of a directory"
    )

    args = parser.parse_args()

    validator = SkillValidator(strict=args.strict)
    path = Path(args.path)

    if args.single:
        if not path.exists():
            print(f"Error: File not found: {path}", file=sys.stderr)
            return 2

        is_valid, errors = validator.validate_file(path)

        if is_valid:
            print(f"✓ {path.name} is valid")
            if errors:  # Warnings only
                print("\nWARNINGS:")
                for error in errors:
                    print(f"  {error}")
        else:
            print(f"✗ {path.name} has errors")
            for error in errors:
                print(f"  {error}")

        return 0 if is_valid else 1
    else:
        success = validator.validate_directory(path)

        if args.strict and validator.warnings:
            print("Strict mode: treating warnings as errors", file=sys.stderr)
            success = False

        return 0 if success else 1


if __name__ == "__main__":
    sys.exit(main())
