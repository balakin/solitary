import type { Route } from './+types/not-found';
import { HomeLayout } from 'fumadocs-ui/layouts/home';
import { baseOptions } from '@/lib/layout.shared';
import { DefaultNotFound } from 'fumadocs-ui/layouts/home/not-found';
import { Footer } from '@/components/footer';

export function meta(_args: Route.MetaArgs) {
  return [{ title: 'Not Found' }];
}

export default function NotFound() {
  return (
    <HomeLayout {...baseOptions()}>
      <DefaultNotFound />
      <Footer />
    </HomeLayout>
  );
}
