// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package kms

import (
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	awstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"
	"github.com/hashicorp/aws-sdk-go-base/v2/awsv1shim/v2/tfawserr"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-provider-aws/internal/sweep"
	"github.com/hashicorp/terraform-provider-aws/internal/sweep/awsv1"
	"github.com/hashicorp/terraform-provider-aws/internal/sweep/sdk"
	"github.com/hashicorp/terraform-provider-aws/internal/tfresource"
)

func RegisterSweepers() {
	resource.AddTestSweepers("aws_kms_key", &resource.Sweeper{
		Name: "aws_kms_key",
		F:    sweepKeys,
	})
}

func sweepKeys(region string) error {
	ctx := sweep.Context(region)
	client, err := sweep.SharedRegionalSweepClient(ctx, region)
	if err != nil {
		return fmt.Errorf("error getting client: %w", err)
	}
	input := &kms.ListKeysInput{
		Limit: aws.Int32(1000),
	}
	conn := client.KMSClient(ctx)
	var sweeperErrs *multierror.Error
	sweepResources := make([]sweep.Sweepable, 0)

	pages := kms.NewListKeysPaginator(conn, input)
	for pages.HasMorePages() {
		page, err := pages.NextPage(ctx)
		if err != nil {
			return err
		}

		for _, v := range page.Keys {
			keyID := aws.ToString(v.KeyId)
			key, err := FindKeyByID(ctx, conn, keyID)

			if tfresource.NotFound(err) {
				continue
			}

			if err != nil {
				if tfawserr.ErrMessageContains(err, "AccessDeniedException", "is not authorized to perform") {
					log.Printf("[DEBUG] Skipping KMS Key (%s): %s", keyID, err)
					continue
				}
				sweeperErrs = multierror.Append(sweeperErrs, fmt.Errorf("reading KMS Key (%s): %w", keyID, err))
				continue
			}

			if key.KeyManager == awstypes.KeyManagerTypeAws {
				log.Printf("[DEBUG] Skipping KMS Key (%s): managed by AWS", keyID)
				continue
			}
			if key.KeyState == awstypes.KeyStatePendingDeletion {
				log.Printf("[DEBUG] Skipping KMS Key (%s): pending deletion", keyID)
				continue
			}

			r := ResourceKey()
			d := r.Data(nil)
			d.SetId(keyID)
			d.Set("key_id", keyID)
			d.Set("deletion_window_in_days", 7) //nolint:gomnd

			sweepResources = append(sweepResources, sdk.NewSweepResource(r, d, client))
		}
	}

	if awsv1.SkipSweepError(err) {
		log.Printf("[WARN] Skipping KMS Key sweep for %s: %s", region, err)
		return sweeperErrs.ErrorOrNil() // In case we have completed some pages, but had errors
	}

	if err != nil {
		sweeperErrs = multierror.Append(sweeperErrs, fmt.Errorf("error listing KMS Keys (%s): %w", region, err))
	}

	err = sweep.SweepOrchestrator(ctx, sweepResources)

	if err != nil {
		sweeperErrs = multierror.Append(sweeperErrs, fmt.Errorf("error sweeping KMS Keys (%s): %w", region, err))
	}

	return sweeperErrs.ErrorOrNil()
}
