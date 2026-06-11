import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../domain/models.dart';
import '../../state/app_providers.dart';
import '../../state/food_search_provider.dart';
import '../../state/scan_provider.dart' show defaultQuantityFor;
import 'photo_confirm.dart';
import 'product_card.dart';

/// The Camera screen's food-search mode: previously-used foods on open, name
/// search as you type, and a quick-create row. Tapping a food opens the shared
/// product card to set quantity + meal type and log it.
class FoodSearchView extends ConsumerStatefulWidget {
  const FoodSearchView({super.key});

  @override
  ConsumerState<FoodSearchView> createState() => _FoodSearchViewState();
}

class _FoodSearchViewState extends ConsumerState<FoodSearchView> {
  final _controller = TextEditingController();

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final state = ref.watch(foodSearchProvider);
    final query = state.query.trim();

    return Column(
      children: [
        Padding(
          padding: const EdgeInsets.fromLTRB(16, 12, 16, 8),
          child: TextField(
            controller: _controller,
            textInputAction: TextInputAction.search,
            onChanged: ref.read(foodSearchProvider.notifier).setQuery,
            decoration: InputDecoration(
              hintText: 'Search foods',
              prefixIcon: const Icon(Icons.search),
              border: const OutlineInputBorder(),
              isDense: true,
              suffixIcon: query.isEmpty
                  ? null
                  : IconButton(
                      icon: const Icon(Icons.clear),
                      onPressed: () {
                        _controller.clear();
                        ref.read(foodSearchProvider.notifier).setQuery('');
                      },
                    ),
            ),
          ),
        ),
        Expanded(
          child: state.results.when(
            loading: () => const Center(child: CircularProgressIndicator()),
            error: (e, _) => _ErrorWithCreate(
              query: query,
              onCreate: () => _createFood(query),
              message: '$e',
            ),
            data: (foods) => RefreshIndicator(
              onRefresh: () => ref.read(foodSearchProvider.notifier).refresh(),
              child: ListView.separated(
                // Always scrollable so pull-to-refresh works even when empty.
                physics: const AlwaysScrollableScrollPhysics(),
                itemCount: foods.length + 1,
                separatorBuilder: (_, _) => const Divider(height: 1),
                itemBuilder: (context, i) {
                  if (i == foods.length) {
                    return _CreateRow(
                      query: query,
                      emphasized: foods.isEmpty,
                      onTap: () => _createFood(query),
                    );
                  }
                  return _FoodTile(
                    food: foods[i],
                    onTap: () => _openLogSheet(foods[i]),
                  );
                },
              ),
            ),
          ),
        ),
      ],
    );
  }

  Future<void> _openLogSheet(Product food) async {
    await showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      builder: (_) => _LogFoodSheet(food: food),
    );
  }

  Future<void> _createFood(String query) async {
    await showFreeformSheet(
      context,
      ref,
      seedName: query.isEmpty ? null : query,
      saveAsProduct: true,
    );
    // A new food may now be the most-recently-used; refresh the list.
    if (mounted) ref.read(foodSearchProvider.notifier).refresh();
  }
}

class _FoodTile extends StatelessWidget {
  final Product food;
  final VoidCallback onTap;
  const _FoodTile({required this.food, required this.onTap});

  @override
  Widget build(BuildContext context) {
    final n = food.nutrimentsPer100g;
    final subtitle = [
      if (food.brand != null && food.brand!.isNotEmpty) food.brand!,
      '${n.kcal.toStringAsFixed(0)} kcal/100g',
    ].join(' · ');
    return ListTile(
      title: Text(food.name, maxLines: 1, overflow: TextOverflow.ellipsis),
      subtitle: Text(subtitle, maxLines: 1, overflow: TextOverflow.ellipsis),
      trailing: const Icon(Icons.add_circle_outline),
      onTap: onTap,
    );
  }
}

class _CreateRow extends StatelessWidget {
  final String query;
  final bool emphasized;
  final VoidCallback onTap;
  const _CreateRow({
    required this.query,
    required this.emphasized,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    final label = query.isEmpty ? 'Create new food' : 'Create "$query"';
    return ListTile(
      leading: Icon(Icons.add,
          color: emphasized ? Theme.of(context).colorScheme.primary : null),
      title: Text(
        label,
        style: emphasized
            ? TextStyle(
                color: Theme.of(context).colorScheme.primary,
                fontWeight: FontWeight.w600)
            : null,
      ),
      subtitle:
          emphasized ? const Text('No matches — add it and log it now') : null,
      onTap: onTap,
    );
  }
}

class _ErrorWithCreate extends StatelessWidget {
  final String query;
  final String message;
  final VoidCallback onCreate;
  const _ErrorWithCreate({
    required this.query,
    required this.message,
    required this.onCreate,
  });

  @override
  Widget build(BuildContext context) {
    return ListView(
      children: [
        const SizedBox(height: 80),
        Center(child: Text('Could not load foods.\n$message',
            textAlign: TextAlign.center)),
        const SizedBox(height: 16),
        _CreateRow(query: query, emphasized: true, onTap: onCreate),
      ],
    );
  }
}

/// Bottom sheet that reuses the scan-flow [ProductCard] to log a food picked
/// from search: quantity pre-filled from `last_logged_quantity_g` →
/// `serving_size_g` → 100, meal type from time of day, committed via the outbox.
class _LogFoodSheet extends ConsumerStatefulWidget {
  final Product food;
  const _LogFoodSheet({required this.food});

  @override
  ConsumerState<_LogFoodSheet> createState() => _LogFoodSheetState();
}

class _LogFoodSheetState extends ConsumerState<_LogFoodSheet> {
  late double _quantityG = defaultQuantityFor(widget.food);
  late String _mealType = mealTypeForNow();

  Future<void> _log() async {
    await ref.read(repositoryProvider).enqueueMeal(
          productId: widget.food.id,
          quantityG: _quantityG,
          mealType: _mealType,
          loggedAt: DateTime.now(),
        );
    if (!mounted) return;
    Navigator.of(context).pop();
    ScaffoldMessenger.of(context)
        .showSnackBar(const SnackBar(content: Text('Meal logged')));
  }

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: EdgeInsets.only(bottom: MediaQuery.of(context).viewInsets.bottom),
      child: ProductCard(
        product: widget.food,
        quantityG: _quantityG,
        mealType: _mealType,
        onQuantityChanged: (q) => setState(() => _quantityG = q),
        onMealTypeChanged: (t) => setState(() => _mealType = t),
        onLog: _log,
        onDismiss: () => Navigator.of(context).pop(),
      ),
    );
  }
}
