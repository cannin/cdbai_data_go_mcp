# CDBAI Data MCP Test Prompts

Each prompt should trigger exactly one call to the `retrieve_data` MCP tool. The expected arguments are included to make manual or automated verification straightforward.

## 1. Gene expression by dataset

> Retrieve up to 10 CCLE gene-expression rows for TP53.

Expected arguments:

```json
{
  "data_types": ["exp"],
  "datasets": ["ccle"],
  "var_names": ["TP53"],
  "limit": 10
}
```

## 2. Known copy-number record

> Retrieve up to 5 CTRP copy-number rows for DDX25 in cell line CVCL_0002.

Expected arguments:

```json
{
  "data_types": ["cop"],
  "datasets": ["ctrp"],
  "obs_names": ["CVCL_0002"],
  "var_names": ["DDX25"],
  "limit": 5
}
```

## 3. Mutation data with a tissue filter

> Retrieve up to 25 CCLE mutation rows for KRAS from lung cell lines.

Expected arguments:

```json
{
  "data_types": ["mut"],
  "datasets": ["ccle"],
  "var_names": ["KRAS"],
  "tissues": ["Lung"],
  "limit": 25
}
```

## 4. RNA-seq data for two genes

> Retrieve up to 50 CCLE RNA-seq expression rows for EGFR and ERBB2 from breast cell lines.

Expected arguments:

```json
{
  "data_types": ["xsq"],
  "datasets": ["ccle"],
  "var_names": ["EGFR", "ERBB2"],
  "tissues": ["Breast"],
  "limit": 50
}
```

## 5. Drug activity by dataset

> Retrieve up to 20 NCI-60 drug-activity rows with non-null values.

Expected arguments:

```json
{
  "data_types": ["act"],
  "datasets": ["nci60"],
  "limit": 20
}
```

## 6. CRISPR measurements

> Retrieve up to 30 Achilles CRISPR rows for TP53 and MDM2.

Expected arguments:

```json
{
  "data_types": ["cri"],
  "datasets": ["achilles"],
  "var_names": ["TP53", "MDM2"],
  "limit": 30
}
```

## 7. DNA methylation by tissue

> Retrieve up to 15 CCLE DNA methylation 450K rows for BRCA1 from ovarian cell lines.

Expected arguments:

```json
{
  "data_types": ["met"],
  "datasets": ["ccle"],
  "var_names": ["BRCA1"],
  "tissues": ["Ovary"],
  "limit": 15
}
```

## 8. Protein measurements across tissues

> Retrieve up to 40 NCI-60 protein rows for AKT1 from breast or lung cell lines.

Expected arguments:

```json
{
  "data_types": ["pro"],
  "datasets": ["nci60"],
  "var_names": ["AKT1"],
  "tissues": ["Breast", "Lung"],
  "limit": 40
}
```

## 9. One variable across datasets

> Retrieve up to 100 TP53 expression rows from either CCLE or NCI-60.

Expected arguments:

```json
{
  "data_types": ["exp"],
  "datasets": ["ccle", "nci60"],
  "var_names": ["TP53"],
  "limit": 100
}
```

## 10. Cell-line-centered retrieval

> Retrieve up to 200 rows for cell line CVCL_0002 from the CTRP dataset, regardless of modality or variable.

Expected arguments:

```json
{
  "datasets": ["ctrp"],
  "obs_names": ["CVCL_0002"],
  "limit": 200
}
```

## 11. Cell-line and gene synonym normalization

> Retrieve up to 10 CCLE gene-expression rows for p53 in the MCF-7 cell line.

Expected arguments:

```json
{
  "data_types": ["exp"],
  "datasets": ["ccle"],
  "obs_names": ["CVCL_0031"],
  "var_names": ["TP53"],
  "limit": 10
}
```

## 12. Alternate gene synonym normalization

> Retrieve up to 10 CCLE gene-expression rows for HER2 in the MCF7 cell line.

Expected arguments:

```json
{
  "data_types": ["exp"],
  "datasets": ["ccle"],
  "obs_names": ["CVCL_0031"],
  "var_names": ["ERBB2"],
  "limit": 10
}
```
