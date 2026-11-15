import { writeFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { bootstrapTemplate } from '../lib/bootstrap/template';
// Explicit local generation only. This command never contacts AWS.
const destination = resolve(__dirname, '../bootstrap/arran-toolkit.template.json');
writeFileSync(destination, `${JSON.stringify(bootstrapTemplate(), null, 2)}\n`);
