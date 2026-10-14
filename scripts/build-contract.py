#!/usr/bin/env python3
"""Build the OpenAPI contract and its TypeScript schema types without network tools."""
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
def ref(name): return {"$ref": "#/components/schemas/" + name}
def array(items): return {"type": "array", "items": items}
def obj(properties, optional=()):
    return {"type": "object", "additionalProperties": False, "properties": properties,
            "required": [k for k in properties if k not in optional]}
string = {"type": "string"}
integer = {"type": "integer"}
boolean = {"type": "boolean"}
date = {"type": "string", "format": "date-time"}
decimal = {"type": "string", "pattern": r"^-?\d+(\.\d+)?$", "description": "Exact decimal encoded as a string; no binary float conversion for calculations."}
rates = {"type": "object", "additionalProperties": decimal}
strings = {"type": "object", "additionalProperties": string}
schemas = {
    "Rates": rates,
    "EffectiveRates": obj({"rates": ref("Rates"), "sources": strings}),
    "CalculationInput": obj({**{k: string for k in ["name", "sku", "category", "notes", "marketMin", "marketMax"]},
       **{k: decimal for k in ["cost", "price", "orders", "fixed", "packaging", "targetMargin"]},
       "overrides": {"type": "object", "additionalProperties": ref("Rates")}}),
    "Metric": obj({"status": {"type": "string", "enum": ["ok", "undefined", "unattainable"]}, "value": {**decimal, "nullable": True}}),
    "Settings": obj({"version": integer, "channels": {"type": "object", "additionalProperties": ref("Rates")},
        "categories": array(string), "categoryReturns": strings, **{k: decimal for k in ["targetMargin", "healthyMargin", "healthyBuffer", "viableMargin"]}, "updatedAt": date}),
    "CategoryRequest": obj({"name": string}),
    "ChannelResult": obj({"channel": string, "rates": ref("Rates"), "sources": strings,
        **{k: decimal for k in ["price", "successfulSales", "revenue", "shippingIncome", "contribution", "contributionPerOrder", "profitPerOrder", "monthlyProfit", "margin"]},
        **{k: ref("Metric") for k in ["breakEvenOrders", "breakEvenPrice", "maxSupplierCost", "priceBuffer", "maxAdvertising", "breakEvenROAS", "targetPrice", "targetCost"]},
        "verdict": {"type": "string", "enum": ["healthy", "viable", "thin", "unprofitable"]}, "reason": string,
        "breakdown": array(obj({"key": string, "value": decimal})), "volume": array(obj({"orders": decimal, "profit": decimal}))}),
    "EvaluationRevision": obj({"id": string, "input": ref("CalculationInput"), "presetVersion": integer, "calculationVersion": string,
        "results": array(ref("ChannelResult")), "recommended": string, "maximumScenario": ref("ChannelResult"), "createdAt": date}, ["maximumScenario"]),
    "Competitor": obj({**{k: string for k in ["id", "platform", "shop", "url"]}, **{k: decimal for k in ["price", "sales", "rating"]}}),
    "ProductImage": obj({**{k: string for k in ["id", "name", "url", "thumbnail"]}, "createdAt": date}),
    "Product": obj({"id": string, "version": integer, "archived": boolean, "input": ref("CalculationInput"),
        "competitors": {**array(ref("Competitor")), "nullable": True}, "images": array(ref("ProductImage")), "revisions": array(ref("EvaluationRevision")), "createdAt": date, "updatedAt": date}),
    "ProductSummary": obj({"id": string, "version": integer, "archived": boolean, "input": ref("CalculationInput"), "createdAt": date, "updatedAt": date,
        "evaluation": ref("EvaluationRevision"), "headline": ref("ChannelResult"), "imageCount": integer, "competitorCount": integer}),
    "SaveRequest": obj({"version": integer, "input": ref("CalculationInput"), "competitors": array(ref("Competitor"))}, ["version", "competitors"]),
    "VersionRequest": obj({"version": integer}),
    "ImportRow": obj({"row": integer, "input": ref("CalculationInput"), "errors": array(string), "evaluation": ref("EvaluationRevision")}, ["evaluation"]),
    "Preview": obj({"id": string, "rows": array(ref("ImportRow")), "createdAt": date, "jobId": string}),
    "ImportUpload": obj({"id": string, "sheets": array(obj({"name": string, "headers": array(string), "rowCount": integer})), "mapping": {"type": "object", "additionalProperties": integer}}),
    "PreviewRequest": obj({"uploadId": string, "sheet": string, "mapping": {"type": "object", "additionalProperties": integer}}),
    "CommitRequest": obj({"previewId": string, "rows": array(integer)}),
    "ExportRequest": obj({"ids": array(string), "filters": strings, "format": {"type": "string", "enum": ["xlsx", "csv"]}, "locale": {"type": "string", "enum": ["en", "id", "zh"]}}, ["ids", "filters"]),
    "JobStatus": obj({"id": string, "kind": {"type": "string", "enum": ["import", "export"]}, "status": {"type": "string", "enum": ["pending", "running", "completed", "failed"]},
        **{k: integer for k in ["total", "processed", "created", "skipped", "failed"]}, "errors": array(string), "createdAt": date, "leaseUntil": date, "artifact": string, "format": string, "locale": string}, ["artifact", "format", "locale"]),
    "ValidationError": obj({"code": string, "message": string, "fields": strings}, ["fields"]),
}
paths = {}
def operation(path, method, name, response=None, request=None, code=200, description="", binary=False, multipart=False, params=()):
    parameters = list(params)
    for part in path.split("/"):
        if part.startswith("{"):
            parameters.append({"in": "path", "name": part[1:-1], "required": True, "schema": string})
    success = {"description": "Success"}
    if response: success["content"] = {"application/json": {"schema": response}}
    if binary: success["content"] = {"application/octet-stream": {"schema": {"type": "string", "format": "binary"}}}
    spec = {"operationId": name, "summary": name, "description": description, "parameters": parameters, "responses": {str(code): success,
        **{str(c): {"description": text, "content": {"application/json": {"schema": ref("ValidationError")}}} for c,text in [(400,"Invalid request"),(404,"Not found"),(409,"Stale version or duplicate SKU"),(422,"Field validation error"),(500,"Server error")]}}}
    if request: spec["requestBody"] = {"required": True, "content": {"multipart/form-data" if multipart else "application/json": {"schema": request}}}
    paths.setdefault(path, {})[method.lower()] = spec

query = lambda name: {"in": "query", "name": name, "schema": string}
operation("/calculate","post","calculate",ref("EvaluationRevision"),ref("CalculationInput"),description="Debounced unsaved evaluation at the entered selling price. Market bounds are context; maximumScenario uses marketMax on the same recommended channel.")
operation("/settings","get","getSettings",ref("Settings"))
operation("/settings","put","updateSettings",ref("Settings"),ref("Settings"),description="Requires current version. Creates a new preset version and leaves saved evaluations untouched.")
operation("/settings/versions","get","getPresetVersions",array(ref("Settings")))
operation("/categories","post","addCategory",ref("Settings"),ref("CategoryRequest"),201,description="Appends a new shared category and saves it as a new preset version. Rejects duplicates case-insensitively.")
operation("/products","get","listProducts",obj({"items": array(ref("ProductSummary")), "total": integer}),params=[query(k) for k in ["search","category","channel","verdict","archived","sort","direction"]])
operation("/products","post","createProduct",ref("Product"),ref("SaveRequest"),201,description="Always calculates on the server. SKU is optional and unique across archived and active products.")
operation("/products/{id}","get","getProduct",ref("Product"))
operation("/products/{id}","put","editProduct",ref("Product"),ref("SaveRequest"),description="Requires current version. Changed calculation inputs create a new evaluation revision.")
operation("/products/{id}/revisions","get","getEvaluationHistory",array(ref("EvaluationRevision")))
for action in ["duplicate","archive","restore","recalculate"]:
    operation("/products/{id}/"+action,"post",action+"Product",ref("Product"),ref("VersionRequest"),description="Requires current product edit version. Explicit recalculation preserves product rate overrides.")
operation("/products/{id}/images","post","uploadProductImage",ref("Product"),obj({"file": {"type":"string","format":"binary"},"version":string}),201,multipart=True,description="JPEG, PNG, WebP up to 5 MB; maximum 5 images. Normalizes to JPEG and produces a thumbnail.")
operation("/products/{id}/images/{image}","delete","deleteProductImage",ref("Product"),ref("VersionRequest"))
operation("/images/{image}","get","getImage",binary=True,description="Use image ID or image ID with -thumb suffix.")
operation("/imports/template","get","downloadImportTemplate",binary=True,params=[query("locale")])
operation("/imports/upload","post","uploadWorkbook",ref("ImportUpload"),obj({"file": {"type":"string","format":"binary"}}),201,multipart=True,description=".xlsx up to 10 MB, 5,000 data rows per sheet, 20 sheets, 200 columns, 64 MB uncompressed. Expires in 24 hours.")
operation("/imports/preview","post","previewImport",ref("Preview"),ref("PreviewRequest"),description="Zero-based source-column indices. Creates no products; formulas and duplicate SKUs are rejected. Valid rows include the same five-channel analysis as /calculate. Source row numbers include the header.")
operation("/imports/commit","post","commitImport",ref("JobStatus"),ref("CommitRequest"),202,description="Select valid source row numbers. Saves the reviewed preview analysis as individual evaluations. Idempotent by preview ID. Worker rechecks SKU uniqueness and checkpoints each committed row.")
operation("/exports","post","createExport",ref("JobStatus"),ref("ExportRequest"),202,description="Explicit IDs or all matching filters. Freezes products and latest saved revisions; never recalculates. Reports expire after 24 hours.")
operation("/jobs/{id}","get","getJobStatus",ref("JobStatus"))
operation("/jobs/{id}/download","get","downloadJobReport",binary=True)
for kind in ["live","ready"]: operation("/health/"+kind,"get",kind+"Health",obj({"status":string}))
contract = {"openapi":"3.0.3", "info":{"title":"Genco Finance API","version":"1.0.0","description":"Shared installation without accounts. All financial calculations are estimated operating profit before tax."},
    "servers":[{"url":"/api/v1"}],"paths":paths,"components":{"schemas":schemas}}
(ROOT / "api/openapi.json").write_text(json.dumps(contract,indent=2,ensure_ascii=False)+"\n")

def ts(schema):
    if "$ref" in schema: value = schema["$ref"].split("/")[-1]
    elif "enum" in schema: value = " | ".join(json.dumps(v) for v in schema["enum"])
    elif schema.get("type") == "array": value = f"Array<{ts(schema['items'])}>"
    elif schema.get("type") == "object":
        if "properties" in schema:
            required=set(schema.get("required",[]))
            value="{ "+"; ".join(k+("" if k in required else "?")+": "+ts(v) for k,v in schema["properties"].items())+" }"
        else: value = f"Record<string, {ts(schema.get('additionalProperties', {}))}>"
    else: value={"integer":"number","number":"number","boolean":"boolean","string":"string"}.get(schema.get("type"),"unknown")
    return value+" | null" if schema.get("nullable") else value
output="// Generated from api/openapi.json by scripts/build-contract.py. Do not edit.\n"
output+="\n".join(f"export type {name} = {ts(schema)}" for name,schema in schemas.items())+"\n"
(ROOT / "frontend/src/generated-api.ts").write_text(output)
print("Generated api/openapi.json and frontend/src/generated-api.ts")
