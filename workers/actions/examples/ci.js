// Approve this JavaScript configuration as the repository owner. The Worker
// stores its immutable compilation; a push cannot rewrite approved commands.
const stepTimeout = 60000;
export default {
  refs: ['refs/heads/main'],
  jobs: [{id: 'test', name: 'Tests', steps: [
    {id: 'unit', name: 'Unit tests', command: 'npm ci && npm test', timeout_ms: stepTimeout},
  ]}],
};
