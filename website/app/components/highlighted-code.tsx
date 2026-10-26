import type { Root } from 'hast';
import { toJsxRuntime } from 'hast-util-to-jsx-runtime';
import { Fragment, jsx, jsxs } from 'react/jsx-runtime';
import { CodeBlock, Pre } from 'fumadocs-ui/components/codeblock';
import { cn } from '@/lib/cn';

/**
 * Renders a tree from highlightCode as fumadocs' code block, copy button and
 * all — the same markup DynamicCodeBlock ends up with, already there on the
 * first paint.
 */
export function HighlightedCode({ tree }: { tree: Root }) {
  return toJsxRuntime(tree, {
    Fragment,
    jsx,
    jsxs,
    components: {
      pre: (props) => (
        <CodeBlock {...props} className={cn('my-0', props.className)}>
          <Pre>{props.children}</Pre>
        </CodeBlock>
      ),
    },
  });
}
