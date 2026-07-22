# CHANGELOG

- Nitin's change for advanced expenses
- Fix validation for files in nested directories
- Add vim mode in editor
- Revamp "Advanced expenses" page with the following:
    - Zoom-In with Context Menu. with context menu having 3 options: 
        - Zoom into subaccounts
        - View all transactions (in a Transaction drawer on the right side)
        - View trend analysis for that account / subaccount
    - And relevant pages for these
- Add gold_etf category for separate taxation
- Add `paisa import <file> --template <name>` CLI command to process CSV/XLSX/XLS statement files using Handlebars import templates and output formatted ledger transactions to stdout
- Add asset allocations and charts in goals
- Fix account glob filtering bug when combining multiple positive and negative patterns
- Fix panic bug
- Add stock dashboard with LTP, tagging, tag filter, target price setting, and long/short term holding breakdown (Nitin)
- Add Kite integration with background tasks and multi-account support
- Add Coin (Zerodha) trades support, with kite/coin order dedup via db and a ledger file prettify command
- Fix UI of goals configuration
- Disable background tasks & Kite/Coin sync
- Fix Yahoo price fetcher
- Add transaction drill-down for Liabilities accounts (click account name in balance table, like Assets) and Expenses accounts (click category label in monthly/yearly breakdown charts)
- Fix crash when navigating to a page without a matching Navbar sub-link (e.g. the new transactions drill-down pages)
- Add an Expenses "Balance" page listing expense accounts in a tree table, like Assets/Liabilities, with each account linking to its transactions
- Update capital gains tax engine for the Jul 2024 budget (new equity 12.5%/20% LTCG/STCG rates, 24 month holding period + no indexation for debt/equity35/unlisted equity), fix Gold ETF missing from the capital gains query, and add Short/Long Term Taxable Gain columns + an expandable ITR-style quarter-wise breakup to the Capital Gains page, all shown with 2 decimal precision
- Fix editor validate (--pedantic/--strict) flagging all accounts as unknown by including accounts.ledger before validating

