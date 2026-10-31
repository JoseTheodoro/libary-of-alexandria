-- A identidade da extração inclui o hash do adaptador; preservá-lo permite
-- verificar a proveniência sem depender da cópia externa do manifesto.
ALTER TABLE extraction_revisions ADD COLUMN adapter_sha256 TEXT;
