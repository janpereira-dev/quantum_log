DELETE FROM budgets
WHERE scope = 'tag'
  AND instr(target, '=') > 0
  AND EXISTS (
      SELECT 1
      FROM budgets AS canonical
      WHERE canonical.scope = 'tag'
        AND instr(canonical.target, '=') > 0
        AND canonical.id <> budgets.id
        AND canonical.id < budgets.id
        AND lower(trim(substr(canonical.target, 1, instr(canonical.target, '=') - 1))) || '=' || lower(trim(substr(canonical.target, instr(canonical.target, '=') + 1))) = lower(trim(substr(budgets.target, 1, instr(budgets.target, '=') - 1))) || '=' || lower(trim(substr(budgets.target, instr(budgets.target, '=') + 1)))
  );

UPDATE budgets
SET target = lower(trim(substr(target, 1, instr(target, '=') - 1))) || '=' || lower(trim(substr(target, instr(target, '=') + 1)))
WHERE scope = 'tag'
  AND instr(target, '=') > 0;
