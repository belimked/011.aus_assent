-- Repair historical phone-sync contacts whose JSON was never projected into columns.
UPDATE contacts SET
  roles = json_extract(data, '$.roles'),
  tags = json_extract(data, '$.tags'),
  flags = json_extract(data, '$.flags'),
  consent = json_extract(data, '$.consent')
WHERE json_valid(data);
