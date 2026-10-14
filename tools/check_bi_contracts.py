#!/usr/bin/env python3
"""Validate native BI contracts and optional synthetic HTTP responses."""
import argparse
import copy
import json
from pathlib import Path
import re

import yaml

from jsonschema import Draft202012Validator, FormatChecker

ROOT = Path(__file__).resolve().parents[1]
FILES = ("openapi.yaml", "admin.openapi.json", "desktop.openapi.yaml", "privacy-notice.openapi.yaml")


def normalize(value):
    if isinstance(value, list):
        return [normalize(item) for item in value]
    if not isinstance(value, dict):
        return value
    result = {key: normalize(item) for key, item in value.items() if key != "nullable"}
    if value.get("nullable"):
        return {"anyOf": [result, {"type": "null"}]}
    return result


def refs(value):
    if isinstance(value, dict):
        if "$ref" in value:
            yield value["$ref"]
        for item in value.values():
            yield from refs(item)
    elif isinstance(value, list):
        for item in value:
            yield from refs(item)


def pointer(document, reference):
    if not reference.startswith("#/"):
        raise ValueError(f"External reference is not supported: {reference}")
    result = document
    for part in reference[2:].split("/"):
        result = result[part.replace("~1", "/").replace("~0", "~")]
    return result


def contracts():
    result = []
    for name in FILES:
        document = yaml.safe_load((ROOT / "docs/mobile-bi" / name).read_text())
        for reference in refs(document):
            pointer(document, reference)
        normalized = normalize(document)
        for schema in normalized["components"]["schemas"].values():
            Draft202012Validator.check_schema(schema)
        ids = []
        for methods in document["paths"].values():
            for operation in methods.values():
                if isinstance(operation, dict) and operation.get("operationId"):
                    ids.append(operation["operationId"])
        if len(ids) != len(set(ids)):
            raise ValueError(f"Duplicate operation IDs: {name}")
        result.append(normalized)
    return result


def validate_response(documents, response):
    for document in documents:
        for template, methods in document["paths"].items():
            pattern = re.sub(r"\\\{[^}]+\\\}", "[^/]+", re.escape(template))
            if not re.fullmatch(pattern, response["path"]):
                continue
            operation = methods.get(response["method"].lower())
            if not operation:
                continue
            statuses = operation["responses"]
            definition = statuses.get(str(response["status"])) or statuses.get("default")
            if not definition:
                raise ValueError(f"Undocumented response status: {response['path']}")
            schema = definition.get("content", {}).get("application/json", {}).get("schema")
            if schema:
                schema = copy.deepcopy(schema)
                schema["components"] = document["components"]
                Draft202012Validator(schema, format_checker=FormatChecker()).validate(response["data"])
            return
    raise ValueError(f"Undocumented response path: {response['path']}")


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--responses", type=Path)
    arguments = parser.parse_args()
    documents = contracts()
    count = 0
    if arguments.responses:
        for response in json.loads(arguments.responses.read_text()):
            validate_response(documents, response)
            count += 1
    print(f"BI contracts validated: {len(documents)} documents, {count} synthetic responses")


if __name__ == "__main__":
    main()
