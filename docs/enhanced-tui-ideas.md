# Enhanced TUI Design Concepts

Based on modern, rich terminal interfaces, here are several design patterns that can be implemented in Mardi Gras using BubbleTea and Lipgloss to elevate the visual experience:

## [x] 1. Embedded Window Titles inside Borders

Instead of using full rows for headers, section titles can be embedded directly within the top border lines (e.g., `┌─ Rolling (3) ─`).

- **Implementation**: Use `lipgloss.Border` with `lipgloss.RoundedBorder()` and position the title text to overlap the top border string.
- **Benefit**: Saves precious vertical space and provides a much cleaner, cohesive visual grouping for the different parade route sections (Rolling, Lined Up, Stalled).

## [x] 2. Block and Braille Character Visualizations

Rich TUIs often use specialized Unicode characters for dense, inline data visualization (e.g., Braille characters `⣿⣦` for sparklines or block characters `[||||||   ]` for progress bars).

- **Implementation**: Introduce a small summary section or visual indicators for Epics/Milestones using these characters to show progress (e.g., how many issues in an Epic are `Rolling` vs `Past the Stand`).
- **Benefit**: Adds visual texture and allows for quick, scannable insights without taking up extensive screen real estate.

## [x] 3. High-Contrast Keybinding Footer

Make the keyboard shortcuts in the footer look like physical buttons or stand out vividly from the help text.

- **Implementation**: Update the bottom bar styling using `lipgloss.NewStyle().Background(...)` and contrasting foreground colors to highlight the actual trigger keys (e.g., `[/] filter`, `[j/k] navigate`, `[tab] pane`).
- **Benefit**: Dramatically improves discoverability and makes the interface feel more like an interactive application rather than a static text dump.

## [x] 4. Multi-color Theming & Gradients

Mardi Gras is named after a vibrant celebration, but the interface can push this further with smooth color transitions, especially for the "parade route" timeline.

- **Implementation**: Use Lipgloss's built-in support for generating color gradients (e.g., transitioning between the traditional Mardi Gras colors: Purple, Gold, and Green). This could be applied to the ASCII art header, or as a subtle background gradient for the currently active/selected row.
- **Benefit**: Delivers the "Joy over minimalism" design principle mentioned in the README, making the tool delightful to stare at every day.
