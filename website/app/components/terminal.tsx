import { cn } from '@/lib/cn';

interface TerminalProps {
  /** The command that produced this screen, shown in the title bar. */
  command: string;
  /**
   * What the screen shows, for anyone not reading it. The art is box-drawing
   * characters, which a screen reader would otherwise announce one by one.
   */
  label: string;
  children: string;
  className?: string;
}

export function Terminal({ command, label, children, className }: TerminalProps) {
  return (
    <figure
      className={cn(
        'overflow-hidden rounded-xl border border-fd-border bg-fd-card shadow-sm',
        className,
      )}
    >
      <figcaption className="flex items-center gap-2 border-b border-fd-border px-4 py-2.5">
        <span aria-hidden className="flex gap-1.5">
          <span className="size-2.5 rounded-full bg-fd-border" />
          <span className="size-2.5 rounded-full bg-fd-border" />
          <span className="size-2.5 rounded-full bg-fd-border" />
        </span>
        <code className="font-mono text-xs text-fd-muted-foreground">{command}</code>
      </figcaption>
      {/* Fixed-width art: it scrolls inside this box rather than widening the page. */}
      <div className="overflow-x-auto p-4">
        {/*
         * leading-[1.2] is load-bearing, not taste: box-drawing verticals only
         * meet across lines while the line box stays inside the glyphs' own
         * extent. A roomier line-height breaks the frame into dashes.
         */}
        {/* The art is a picture made of text, so it is one image to a screen
            reader rather than a wall of box-drawing characters — an <img> tag
            cannot hold the text itself. */}
        {/* oxlint-disable-next-line jsx-a11y/prefer-tag-over-role */}
        <pre role="img" aria-label={label} className="w-fit font-mono text-[0.8rem] leading-[1.2]">
          {children}
        </pre>
      </div>
    </figure>
  );
}
