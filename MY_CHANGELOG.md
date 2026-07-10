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

