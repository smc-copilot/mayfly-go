import { defineStore } from 'pinia';
export const useCasdoorContext = defineStore('casdoorServiceContext', {
    state: () => ({ serviceId: 0, organizationId: 0 }),
});
