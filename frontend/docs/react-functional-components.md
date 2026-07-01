# React components in this app

All UI under `frontend/src/**/*.tsx` uses functional components with hooks.

There are no `React.Component` or `class ... extends Component` patterns in the frontend tree. Non-React TypeScript classes (for example keyword classifiers under `frontend/src/ai/`) are plain domain objects and are intentionally excluded from this rule.

When adding UI:

- Prefer function components with `useState`, `useEffect`, and `useCallback`.
- Keep side effects out of render paths.
- Run `python -m unittest tools.tests.test_functional_react_components -v` before opening a PR.
