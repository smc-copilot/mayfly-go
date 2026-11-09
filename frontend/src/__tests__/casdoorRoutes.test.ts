import { existsSync, readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

describe('Casdoor menu route registration', () => {
    it('registers the connection page against a real component', () => {
        const root = resolve(process.cwd(), 'src/views/casdoor');
        const routeFile = resolve(root, 'route.ts');
        expect(existsSync(routeFile)).toBe(true);
        expect(readFileSync(routeFile, 'utf8')).toContain('CasdoorConnectionList');
        expect(existsSync(resolve(root, 'connections/ConnectionList.vue'))).toBe(true);
    });

    it('registers independent application and user pages', () => {
        const root = resolve(process.cwd(), 'src/views/casdoor');
        const routes = readFileSync(resolve(root, 'route.ts'), 'utf8');
        expect(routes).toContain('CasdoorApplicationList');
        expect(routes).toContain('CasdoorUserList');
        expect(existsSync(resolve(root, 'applications/ApplicationList.vue'))).toBe(true);
        expect(existsSync(resolve(root, 'users/UserList.vue'))).toBe(true);
    });

    it('registers the operations page', () => {
		const root = resolve(process.cwd(), 'src/views/casdoor');
		expect(readFileSync(resolve(root, 'route.ts'), 'utf8')).toContain('CasdoorOperationList');
		expect(existsSync(resolve(root, 'operations/OperationList.vue'))).toBe(true);
    });

    it('registers the access page', () => {
        const root = resolve(process.cwd(), 'src/views/casdoor');
        expect(readFileSync(resolve(root, 'route.ts'), 'utf8')).toContain('CasdoorAccessManagement');
        expect(existsSync(resolve(root, 'access/AccessManagement.vue'))).toBe(true);
    });
});
